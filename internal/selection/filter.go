package selection

import (
	"path/filepath"
	"strings"
	"time"

	"tbmux/internal/model"
)

func ApplyFilter(runs []model.RunRecord, f model.Filter, now time.Time) []model.RunRecord {
	out := make([]model.RunRecord, 0, len(runs))
	for _, run := range runs {
		if !matchOne(run, f, now) {
			continue
		}
		out = append(out, run)
	}
	return out
}

func matchOne(run model.RunRecord, f model.Filter, now time.Time) bool {
	if f.Today {
		y1, m1, d1 := now.Local().Date()
		y2, m2, d2 := run.LastUpdatedAt.Local().Date()
		if y1 != y2 || m1 != m2 || d1 != d2 {
			return false
		}
	}
	if f.Hours > 0 && now.Sub(run.LastUpdatedAt) > time.Duration(f.Hours)*time.Hour {
		return false
	}
	if f.Days > 0 && now.Sub(run.LastUpdatedAt) > time.Duration(f.Days)*24*time.Hour {
		return false
	}
	if f.RunningOnly != nil {
		if run.IsRunning != *f.RunningOnly {
			return false
		}
	}
	if f.Under != "" {
		under := filepath.Clean(f.Under)
		src := filepath.Clean(run.SourcePath)
		if src != under && !strings.HasPrefix(src, under+string(filepath.Separator)) {
			return false
		}
	}
	if f.Match != "" {
		q := strings.ToLower(f.Match)
		hay := strings.ToLower(run.Name + " " + run.ID + " " + run.SourcePath)
		if !strings.Contains(hay, q) {
			return false
		}
	}
	return true
}
