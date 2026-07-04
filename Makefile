BINARY    := cyberstrike-ai
CONFIG    := config.yaml
PID_FILE  := .cyberstrike.pid
LOG_FILE  := cyberstrike.log
VENV_DIR  := venv
DEV_BRANCH := main-llf

ifdef HTTP
  HTTPS_FLAG :=
else
  HTTPS_FLAG := --https
endif

.PHONY: build start stop restart status logs clean setup air sync push pull commit diff log

build:
	@echo ">>> Building $(BINARY)..."
	@go build -o $(BINARY) cmd/server/main.go
	@echo ">>> Build complete"

setup:
	@if [ ! -d "$(VENV_DIR)" ]; then \
		echo ">>> Creating Python venv..."; \
		python3 -m venv $(VENV_DIR); \
	fi
	@echo ">>> Installing Python dependencies..."
	@. $(VENV_DIR)/bin/activate && pip install -r requirements.txt -q
	@echo ">>> Setup complete"

start: build
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo ">>> Already running (PID: $$(cat $(PID_FILE)))"; \
		exit 1; \
	fi
	@echo ">>> Starting $(BINARY) in background..."
	@nohup ./$(BINARY) -config $(CONFIG) $(HTTPS_FLAG) > $(LOG_FILE) 2>&1 & echo $$! > $(PID_FILE)
	@sleep 1
	@if kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo ">>> Started (PID: $$(cat $(PID_FILE)))"; \
		echo ">>> Log: $(LOG_FILE)"; \
	else \
		echo ">>> Failed to start, check $(LOG_FILE)"; \
		rm -f $(PID_FILE); \
		exit 1; \
	fi

stop:
	@if [ ! -f $(PID_FILE) ]; then \
		PID=$$(pgrep -f "./$(BINARY)" 2>/dev/null); \
		if [ -n "$$PID" ]; then \
			echo ">>> Stopping (PID: $$PID)..."; \
			kill $$PID; \
			echo ">>> Stopped"; \
		else \
			echo ">>> Not running"; \
		fi; \
		exit 0; \
	fi
	@PID=$$(cat $(PID_FILE)); \
	if kill -0 $$PID 2>/dev/null; then \
		echo ">>> Stopping (PID: $$PID)..."; \
		kill $$PID; \
		sleep 2; \
		if kill -0 $$PID 2>/dev/null; then \
			echo ">>> Force killing..."; \
			kill -9 $$PID; \
		fi; \
		echo ">>> Stopped"; \
	else \
		echo ">>> Process not running (stale PID file)"; \
	fi; \
	rm -f $(PID_FILE)

restart: stop
	@sleep 1
	@$(MAKE) start

status:
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo ">>> Running (PID: $$(cat $(PID_FILE)))"; \
	else \
		echo ">>> Not running"; \
		rm -f $(PID_FILE) 2>/dev/null; \
	fi

logs:
	@if [ -f $(LOG_FILE) ]; then \
		tail -f $(LOG_FILE); \
	else \
		echo ">>> No log file found"; \
	fi

## 热加载开发模式（需要 air: go install github.com/air-verse/air@latest）
air:
	@AIR=$$(go env GOPATH)/bin/air; \
	if [ ! -f "$$AIR" ]; then \
		echo ">>> Installing air..."; \
		go install github.com/air-verse/air@latest; \
	fi; \
	echo ">>> Starting hot-reload dev mode..."; \
	exec "$$AIR"

## 同步上游仓库更新到开发分支
sync:
	@echo ">>> Fetching upstream..."
	@git fetch upstream
	@CURRENT=$$(git branch --show-current); \
	if [ "$$CURRENT" != "$(DEV_BRANCH)" ]; then \
		echo ">>> Switching to $(DEV_BRANCH)..."; \
		git checkout $(DEV_BRANCH); \
	fi
	@echo ">>> Merging upstream/main into $(DEV_BRANCH)..."
	@git merge upstream/main --no-edit || (echo ">>> Conflict detected, resolve manually then run: git add . && git commit"; exit 1)
	@echo ">>> Sync complete"

## 同步 fork 的 main 分支与上游一致
sync-main:
	@echo ">>> Syncing origin/main with upstream/main..."
	@git fetch upstream
	@git push origin upstream/main:main
	@echo ">>> origin/main synced"

## 推送 main-llf 到 origin
push:
	@echo ">>> Pushing $(DEV_BRANCH) to origin..."
	@git push -u origin $(DEV_BRANCH)
	@echo ">>> Push complete"

## 拉取远程 main-llf 的更新
pull:
	@echo ">>> Pulling $(DEV_BRANCH) from origin..."
	@git pull origin $(DEV_BRANCH)
	@echo ">>> Pull complete"

## 快速提交所有变更,快速 add + commit（用法：make commit m="提交信息"）
commit:
	@if [ -z "$(m)" ]; then \
		echo ">>> Usage: make commit m=\"your commit message\""; \
		exit 1; \
	fi
	@git add -A
	@git commit -m "$(m)"
	@echo ">>> Committed: $(m)"

## 查看当前改动
diff:
	@git diff --stat
	@echo ""
	@git status -s

## 查看开发分支相对于上游的所有提交
log:
	@echo ">>> Commits on $(DEV_BRANCH) ahead of upstream/main:"
	@git log --oneline upstream/main..$(DEV_BRANCH)

## 查看分支状态总览(领先/落后多少)
branch:
	@echo ">>> Current branch:"
	@git branch --show-current
	@echo ""
	@echo ">>> Branch status:"
	@git branch -vv
	@echo ""
	@echo ">>> Ahead of upstream/main: $$(git rev-list --count upstream/main..HEAD) commits"
	@echo ">>> Behind upstream/main:   $$(git rev-list --count HEAD..upstream/main) commits"

clean:
	@rm -f $(BINARY) $(PID_FILE) $(LOG_FILE)
	@rm -rf tmp
	@echo ">>> Cleaned"
