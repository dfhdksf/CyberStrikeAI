package handler

import (
	"bytes"
	"cyberstrike-ai/internal/database"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRBACAssignResourceBatchIsAtomicAndLegacyCompatible(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "rbac-handler.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	user, err := db.CreateRBACUser("api-member", "API Member", "hash", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	p1, _ := db.CreateProject(&database.Project{Name: "p1"})
	p2, _ := db.CreateProject(&database.Project{Name: "p2"})
	p3, _ := db.CreateProject(&database.Project{Name: "p3"})

	h := NewRBACHandler(db, zap.NewNop())
	router := gin.New()
	router.POST("/api/rbac/resource-assignments", h.AssignResource)

	batch := performRBACJSONRequest(t, router, map[string]interface{}{
		"user_id": user.ID, "resource_type": "project", "resource_ids": []string{p1.ID, p2.ID},
	})
	if batch.Code != http.StatusOK {
		t.Fatalf("batch status = %d, body = %s", batch.Code, batch.Body.String())
	}
	var batchBody map[string]interface{}
	if err := json.Unmarshal(batch.Body.Bytes(), &batchBody); err != nil {
		t.Fatal(err)
	}
	if batchBody["created"] != float64(2) {
		t.Fatalf("batch response = %#v, want created=2", batchBody)
	}

	invalid := performRBACJSONRequest(t, router, map[string]interface{}{
		"user_id": user.ID, "resource_type": "project", "resource_ids": []string{p3.ID, "missing"},
	})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", invalid.Code, invalid.Body.String())
	}
	rows, err := db.ListRBACResourceAssignments(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("failed batch persisted partial data: %#v", rows)
	}

	legacy := performRBACJSONRequest(t, router, map[string]interface{}{
		"user_id": user.ID, "resource_type": "project", "resource_id": p3.ID,
	})
	if legacy.Code != http.StatusOK {
		t.Fatalf("legacy status = %d, body = %s", legacy.Code, legacy.Body.String())
	}
}

func TestRBACAssignableResourcesArePaged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.NewDB(filepath.Join(t.TempDir(), "rbac-picker.db"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, name := range []string{"p1", "p2", "p3"} {
		if _, err := db.CreateProject(&database.Project{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	h := NewRBACHandler(db, zap.NewNop())
	router := gin.New()
	router.GET("/api/rbac/resources", h.ListAssignableResources)

	request := httptest.NewRequest(http.MethodGet, "/api/rbac/resources?type=project&limit=2&offset=0", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Resources []database.RBACResourceOption `json:"resources"`
		HasMore   bool                          `json:"has_more"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Resources) != 2 || !body.HasMore {
		t.Fatalf("page = %#v, has_more = %v; want two rows and another page", body.Resources, body.HasMore)
	}
}

func performRBACJSONRequest(t *testing.T, router http.Handler, payload map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/rbac/resource-assignments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
