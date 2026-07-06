# 知识库检索优化方案

## 当前现状

- 检索方式：纯语义向量检索（text-embedding-3-small）
- 存储：SQLite 全量加载到内存计算余弦相似度
- 流程：MultiQuery 扩展 → 向量检索 → Rerank → 去重 → Top-5
- 瓶颈：体量增大后全量扫描慢；单一语义通道对精确关键词（payload、函数名）召回不佳

---

## 优化目标：动态权重 + RRF + Reranker 三级混合检索

### 整体架构

```
用户查询
    ↓
查询分类器（判断查询类型）
    ↓
确定动态权重 (vectorW, bm25W)
    ↓
┌────────────────────────────────────────┐
│          两路并行召回（各 10 条）         │
│                                        │
│  向量检索（语义相似）  BM25（关键词匹配）  │
│       10 条                10 条        │
└───────────┬────────────────┬───────────┘
            ↓                ↓
        去重合并（7~20 条，取决于重叠度）
            ↓
    加权 RRF 融合打分 + 排序
            ↓
      取 Top-15（Reranker 候选）
            ↓
      Reranker 精排（交叉编码器）
            ↓
        Top-5 返回给 Agent
```

### 数量变化

| 阶段 | 数量 | 作用 |
|------|------|------|
| 向量召回 | 10 | 语义相似候选 |
| BM25 召回 | 10 | 关键词匹配候选 |
| 去重合并 | 7~20 | 取决于两路重叠度 |
| RRF 融合排序 | 全部 | 加权融合打分 |
| Reranker 候选 | 15 | 砍掉尾部，控制精排成本 |
| Reranker 精排 | 15→5 | 交叉编码器逐条评分 |
| 最终返回 | 5 | Agent 拿到的结果 |

---

## 各组件职责

### 1. 动态权重（融合前）

根据查询特征偏向更可能有效的通道：

```go
func dynamicWeight(query string) (vectorW, bm25W float64) {
    // 包含确切 payload/函数名 → BM25 权重提高
    if containsExactPayload(query) {  // "' OR 1=1--", "pg_sleep", "UNION SELECT"
        return 0.3, 0.7
    }
    // 自然语言描述性查询 → 向量权重提高
    if isNaturalLanguageQuery(query) {  // "如何绕过WAF", "提权方法"
        return 0.7, 0.3
    }
    // 混合型 → 均衡
    return 0.5, 0.5
}
```

**判定规则**：
- 含特殊字符（`'`, `--`, `()`, SQL 关键字原文）→ 偏 BM25
- 纯中文/英文自然语言描述 → 偏向量
- 混合（关键词 + 描述）→ 均衡

### 2. RRF 融合（融合时）

Reciprocal Rank Fusion，只看排名不看分数，天然抗尺度差异：

```go
// RRF 公式：score(d) = w_vec * 1/(k + rank_vec(d)) + w_bm25 * 1/(k + rank_bm25(d))
// k = 60（标准值）
// 只在一路中出现的文档，另一路贡献为 0

func reciprocalRankFusion(vectorResults, bm25Results []Doc, vectorW, bm25W float64, k int) []ScoredDoc {
    scores := map[string]float64{}
    for rank, doc := range vectorResults {
        scores[doc.ID] += vectorW * 1.0 / float64(k + rank + 1)
    }
    for rank, doc := range bm25Results {
        scores[doc.ID] += bm25W * 1.0 / float64(k + rank + 1)
    }
    // 按融合分数降序排列
    return sortByScore(scores)
}
```

**示例计算**：
```
文档同时在向量排第5、BM25排第1（权重各0.5）：
  score = 0.5 * 1/(60+5) + 0.5 * 1/(60+1) = 0.0077 + 0.0082 = 0.0159

文档仅在向量排第1（BM25未命中）：
  score = 0.5 * 1/(60+1) + 0 = 0.0082
```

两路共同命中的文档得分更高，但单路高排名文档也能进入候选。

### 3. Reranker 精排（融合后）

对候选集中每个 (query, document) 对做精确语义判断：

```go
func rerank(query string, candidates []Doc, topK int) []Doc {
    // 调用 Reranker API（Cohere / bge-reranker / DashScope）
    // 输入：query + 15 条候选文本
    // 输出：每条一个 0~1 的相关性分数
    scores := rerankAPI.Rerank(query, candidates)
    sort.Slice(candidates, func(i, j int) bool {
        return scores[i] > scores[j]
    })
    return candidates[:topK]  // Top-5
}
```

**Reranker 有权翻盘**：即使某条文档在 RRF 中排第 14，Reranker 仍可能提到第 1。

---

## 两路结果差异大时的处理策略

| 场景 | 表现 | 处理 |
|------|------|------|
| 向量好 BM25 差 | 查询"如何绕过认证" → 向量找到"身份验证绕过"，BM25 无精确词匹配 | 动态权重偏向量；Reranker 自然过滤 BM25 噪声 |
| BM25 好向量差 | 查询"pg_sleep(5)" → BM25 精确命中，向量理解为"睡眠函数" | 动态权重偏 BM25；Reranker 确认精确匹配更相关 |
| 两路都好但不同 | 各自找到不同维度的相关文档 | RRF 保留两路各自的 Top 文档；Reranker 从中选最佳 |
| 重叠极低（Jaccard<0.1） | 两路在"回答不同子问题" | 保留两路各自 Top-3，让 Agent 自行判断 |

---

## 工程实现路径

### Phase 1（短期）— BM25 混合

- [ ] 引入 BM25 引擎（bleve 或 tantivy-go 绑定）
- [ ] 索引阶段同时构建 BM25 倒排索引
- [ ] 实现 RRF 融合函数
- [ ] 复用现有 Reranker（`internal/knowledge/rerank_http.go`）做精排
- [ ] 查询分类器（正则 + 简单规则）确定动态权重

### Phase 2（中期）— 索引优化

- [ ] 向量索引替换为 sqlite-vss（HNSW 算法），ANN 近似搜索
- [ ] 按 risk_type 分区索引，缩小搜索范围
- [ ] 增量索引（只处理变更文档，不全量重建）
- [ ] Reranker 候选数可配置化（当前 15，可根据延迟调整）

### Phase 3（大规模）— 架构升级

- [ ] 独立向量数据库（Milvus / Qdrant）替代 SQLite
- [ ] HyDE（Hypothetical Document Embedding）：LLM 先生成假设性文档再做向量检索
- [ ] 查询路由：简单查询直接 BM25，复杂查询走混合
- [ ] 检索结果缓存（相同 query + risk_type 短期缓存）

---

## 设计原则

1. **初检负责"不漏"**：两路各出 10 条，宁可多不可少
2. **RRF 负责"合理排队"**：统一排名尺度，两路共识的排前面
3. **Reranker 负责"选对"**：交叉编码器精确判断，有权翻盘初检排序
4. **三者是逐层收敛的漏斗**：20 → 15 → 5，每层有明确的筛选标准
