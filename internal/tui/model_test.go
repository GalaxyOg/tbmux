package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"tbmux/internal/app"
	"tbmux/internal/config"
	"tbmux/internal/model"
)

func testModel() Model {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	st := model.State{
		Version: 1,
		Discovered: []model.RunRecord{
			{ID: "r1", Name: "run_one", SourcePath: "/tmp/r1", WatchRoot: "/tmp", LastUpdatedAt: now, IsRunning: true},
		},
		Selected: map[string]model.SelectionEntry{},
	}
	return New("/tmp/config.toml", config.Config{}, st, model.Filter{})
}

func TestToggleSelectionMakesDirty(t *testing.T) {
	m := testModel()
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m2 := out.(Model)
	if !m2.dirty {
		t.Fatalf("expected dirty after toggle")
	}
	if _, ok := m2.draftSelected["r1"]; !ok {
		t.Fatalf("expected r1 selected")
	}
}

func TestQuitWithDirtyNeedsConfirm(t *testing.T) {
	m := testModel()
	m.dirty = true
	out, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m2 := out.(Model)
	if !m2.exitConfirmMode || m2.quitting {
		t.Fatalf("expected enter confirm mode before quit")
	}
	out2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m3 := out2.(Model)
	if !m3.quitting {
		t.Fatalf("expected quit on second q")
	}
}

func TestApplyCmdPersistsSelectionAndSymlink(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "run")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Managed: config.Managed{
			RunDir:       filepath.Join(dir, "managed"),
			StatePath:    filepath.Join(dir, "state", "state.json"),
			PidPath:      filepath.Join(dir, "state", "tb.pid"),
			LogPath:      filepath.Join(dir, "state", "tb.log"),
			CleanupStale: true,
		},
		Scan:        config.Scan{RunningWindowMinutes: 15},
		TensorBoard: config.TensorBoard{Host: "127.0.0.1", Port: 6006},
	}
	st := model.State{
		Version: 1,
		Discovered: []model.RunRecord{
			{ID: "r1", Name: "run_one", SourcePath: src, WatchRoot: dir, LastUpdatedAt: time.Now().UTC(), IsRunning: true},
		},
		Selected: map[string]model.SelectionEntry{},
	}
	m := New(filepath.Join(dir, "config.toml"), cfg, st, model.Filter{})
	m.draftSelected["r1"] = model.SelectionEntry{Source: "test", SelectedAt: time.Now().UTC()}
	cmd := m.applyCmd()
	msg := cmd().(appliedMsg)
	if msg.err != nil {
		t.Fatalf("apply failed: %v", msg.err)
	}
	if msg.applied != 1 {
		t.Fatalf("expected applied=1 got %d", msg.applied)
	}
	loaded, err := app.LoadState(cfg)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if len(loaded.Selected) != 1 {
		t.Fatalf("expected selected persisted")
	}
	link := filepath.Join(cfg.Managed.RunDir, "selected", "run_one")
	if _, err := os.Readlink(link); err != nil {
		t.Fatalf("expected symlink at %s: %v", link, err)
	}
}
