package runs

import (
	"fmt"
	"os"
	"path/filepath"

	"tbmux/internal/model"
)

func SelectedDir(runDir string) string {
	return filepath.Join(runDir, "selected")
}

func ApplySelected(runDir string, selected []model.RunRecord, cleanupStale bool) (int, error) {
	targetDir := SelectedDir(runDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return 0, err
	}
	if cleanupStale {
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			return 0, err
		}
		for _, e := range entries {
			if err := os.RemoveAll(filepath.Join(targetDir, e.Name())); err != nil {
				return 0, err
			}
		}
	}
	nameCount := map[string]int{}
	applied := 0
	for _, run := range selected {
		name := run.Name
		if n := nameCount[name]; n > 0 {
			name = fmt.Sprintf("%s__%d", name, n+1)
		}
		nameCount[run.Name]++
		linkPath := filepath.Join(targetDir, name)
		if _, err := os.Lstat(linkPath); err == nil {
			if err := os.Remove(linkPath); err != nil {
				return applied, err
			}
		}
		if err := os.Symlink(run.SourcePath, linkPath); err != nil {
			return applied, err
		}
		applied++
	}
	return applied, nil
}
