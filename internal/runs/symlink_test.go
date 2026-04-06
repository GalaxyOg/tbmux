package runs

import (
	"os"
	"path/filepath"
	"testing"

	"tbmux/internal/model"
)

func TestApplySelectedCreatesSymlinks(t *testing.T) {
	base := t.TempDir()
	src1 := filepath.Join(base, "src", "run1")
	src2 := filepath.Join(base, "src", "run2")
	if err := os.MkdirAll(src1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src2, 0o755); err != nil {
		t.Fatal(err)
	}
	selected := []model.RunRecord{
		{ID: "a1", Name: "run", SourcePath: src1},
		{ID: "b2", Name: "run", SourcePath: src2},
	}
	n, err := ApplySelected(filepath.Join(base, "managed"), selected, true)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 links got %d", n)
	}
	entries, err := os.ReadDir(SelectedDir(filepath.Join(base, "managed")))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries got %d", len(entries))
	}
	for _, e := range entries {
		p := filepath.Join(SelectedDir(filepath.Join(base, "managed")), e.Name())
		if _, err := os.Readlink(p); err != nil {
			t.Fatalf("entry should be symlink: %s", p)
		}
	}
}

func TestApplySelectedCleanup(t *testing.T) {
	base := t.TempDir()
	runDir := filepath.Join(base, "managed")
	targetDir := SelectedDir(runDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "stale.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(base, "src", "run")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySelected(runDir, []model.RunRecord{{ID: "a", Name: "run", SourcePath: src}}, true); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected stale entry to be removed, entries=%d", len(entries))
	}
}
