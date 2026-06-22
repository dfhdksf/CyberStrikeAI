package database

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestDeleteConversationRemovesEinoScopedDirs(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "conversations.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	plantaskBase := filepath.Join(tmp, "skills", ".eino", "plantask")
	checkpointBase := filepath.Join(tmp, "eino-checkpoints")
	reductionBase := filepath.Join(tmp, "reduction")
	db.SetEinoConversationDirs(plantaskBase, checkpointBase, reductionBase)

	conv, err := db.CreateConversation("cleanup test", ConversationCreateMeta{})
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	convID := conv.ID
	seg := sanitizeConversationPathSegment(convID)
	for _, base := range []struct {
		root string
		file string
	}{
		{db.conversationArtifactsDir, "transcript.txt"},
		{plantaskBase, "task-1.json"},
		{checkpointBase, "runner-deep.ckpt"},
		{filepath.Join(reductionBase, "conversations"), "tool-output.txt"},
	} {
		dir := filepath.Join(base.root, seg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, base.file), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", base.file, err)
		}
	}

	if err := db.DeleteConversation(convID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}

	for _, base := range []string{db.conversationArtifactsDir, plantaskBase, checkpointBase, filepath.Join(reductionBase, "conversations")} {
		dir := filepath.Join(base, seg)
		if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
			t.Fatalf("expected removed dir %s, stat err=%v", dir, statErr)
		}
	}
}

func TestDeleteProjectRemovesReductionDir(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "conversations.db")
	db, err := NewDB(dbPath, zap.NewNop())
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	reductionBase := filepath.Join(tmp, "reduction")
	db.SetEinoConversationDirs("", "", reductionBase)

	project, err := db.CreateProject(&Project{Name: "cleanup test"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	seg := sanitizeConversationPathSegment(project.ID)
	reductionDir := filepath.Join(reductionBase, "projects", seg, "clear")
	if err := os.MkdirAll(reductionDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", reductionDir, err)
	}
	if err := os.WriteFile(filepath.Join(reductionDir, "call-1.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := db.DeleteProject(project.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	projectReductionDir := filepath.Join(reductionBase, "projects", seg)
	if _, statErr := os.Stat(projectReductionDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected removed dir %s, stat err=%v", projectReductionDir, statErr)
	}
}
