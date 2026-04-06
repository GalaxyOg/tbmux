package selection

import (
	"testing"
	"time"

	"tbmux/internal/model"
)

func sampleState(now time.Time) model.State {
	return model.State{
		Version: 1,
		Discovered: []model.RunRecord{
			{ID: "a1", Name: "llama_train", SourcePath: "/data/a", LastUpdatedAt: now.Add(-1 * time.Hour), IsRunning: true},
			{ID: "b2", Name: "bert_eval", SourcePath: "/data/b", LastUpdatedAt: now.Add(-30 * time.Hour), IsRunning: false},
			{ID: "c3", Name: "vision", SourcePath: "/mnt/c", LastUpdatedAt: now.Add(-2 * time.Hour), IsRunning: true},
		},
		Selected: map[string]model.SelectionEntry{},
	}
}

func TestApplyFilter(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	st := sampleState(now)
	f := model.Filter{Hours: 3, RunningOnly: boolPtr(true), Match: "llama"}
	r := ApplyFilter(st.Discovered, f, now)
	if len(r) != 1 || r[0].ID != "a1" {
		t.Fatalf("unexpected filter result: %+v", r)
	}
}

func TestAddRemoveAndByFilter(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	st := sampleState(now)
	if n, err := AddByTokens(&st, []string{"a1", "vision"}, "manual_add"); err != nil || n != 2 {
		t.Fatalf("add failed n=%d err=%v", n, err)
	}
	if _, ok := st.Selected["a1"]; !ok {
		t.Fatalf("a1 should be selected")
	}
	if n, err := RemoveByTokens(&st, []string{"a1"}); err != nil || n != 1 {
		t.Fatalf("remove failed n=%d err=%v", n, err)
	}
	f := model.Filter{Under: "/data"}
	n := SelectByFilter(&st, f, "set")
	if n != 2 {
		t.Fatalf("expected set=2 got %d", n)
	}
	if len(st.Selected) != 2 {
		t.Fatalf("selected len should be 2 got %d", len(st.Selected))
	}
}

func TestPruneSelected(t *testing.T) {
	st := model.State{
		Version:    1,
		Discovered: []model.RunRecord{{ID: "x1"}},
		Selected: map[string]model.SelectionEntry{
			"x1": {Source: "manual"},
			"z9": {Source: "manual"},
		},
	}
	removed := PruneSelected(&st)
	if removed != 1 {
		t.Fatalf("expected removed=1 got %d", removed)
	}
	if len(st.Selected) != 1 {
		t.Fatalf("expected selected=1 got %d", len(st.Selected))
	}
}

func boolPtr(v bool) *bool { return &v }
