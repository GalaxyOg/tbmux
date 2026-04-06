package discovery

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tbmux/internal/config"
)

func writeEvent(t *testing.T, path string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverAndRunningInference(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	root := t.TempDir()
	recent := filepath.Join(root, "p1", "run1", "events.out.tfevents.1")
	old := filepath.Join(root, "p2", "run2", "events.out.tfevents.2")
	writeEvent(t, recent, now.Add(-10*time.Minute))
	writeEvent(t, old, now.Add(-2*time.Hour))

	s := NewScanner(nil, 30*time.Minute)
	s.Now = func() time.Time { return now }
	runs, err := s.Discover([]config.WatchedRoot{{Path: root, Alias: "data"}})
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	if !runs[0].IsRunning {
		t.Fatalf("latest run should be running")
	}
	if runs[1].IsRunning {
		t.Fatalf("old run should not be running")
	}
}

func TestNamingCollisionGetsSuffix(t *testing.T) {
	now := time.Now().UTC()
	root1 := filepath.Join(t.TempDir(), "r1")
	root2 := filepath.Join(t.TempDir(), "r2")
	writeEvent(t, filepath.Join(root1, "proj", "run", "events.out.tfevents.1"), now)
	writeEvent(t, filepath.Join(root2, "proj", "run", "events.out.tfevents.2"), now)

	s := NewScanner(nil, time.Hour)
	runs, err := s.Discover([]config.WatchedRoot{{Path: root1, Alias: "exp"}, {Path: root2, Alias: "exp"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	if runs[0].Name == runs[1].Name {
		t.Fatalf("name collision should be resolved: %q", runs[0].Name)
	}
}

func TestExcludePattern(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	writeEvent(t, filepath.Join(root, "ok", "events.out.tfevents.1"), now)
	writeEvent(t, filepath.Join(root, ".git", "events.out.tfevents.2"), now)

	s := NewScanner([]string{"*/.git/*"}, time.Hour)
	runs, err := s.Discover([]config.WatchedRoot{{Path: root, Alias: "r"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run after exclude, got %d", len(runs))
	}
}
