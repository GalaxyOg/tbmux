package app

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tbmux/internal/config"
	"tbmux/internal/discovery"
	"tbmux/internal/model"
	"tbmux/internal/runs"
	"tbmux/internal/selection"
	"tbmux/internal/state"
)

type SyncResult struct {
	Discovered int `json:"discovered"`
	Selected   int `json:"selected"`
	Pruned     int `json:"pruned_selected"`
}

func ResolveConfigPath(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	return config.DefaultConfigPath()
}

func EnsureDirs(cfg config.Config) error {
	paths := []string{
		filepath.Dir(cfg.Managed.StatePath),
		filepath.Dir(cfg.Managed.PidPath),
		cfg.Managed.RunDir,
	}
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func LoadConfig(configPath string) (config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return config.Config{}, fmt.Errorf("加载配置失败: %w", err)
	}
	return cfg, nil
}

func LoadState(cfg config.Config) (model.State, error) {
	st, err := state.Load(cfg.Managed.StatePath)
	if err != nil {
		return model.State{}, fmt.Errorf("加载 state 失败: %w", err)
	}
	return st, nil
}

func SaveState(cfg config.Config, st model.State) error {
	if err := state.Save(cfg.Managed.StatePath, st); err != nil {
		return fmt.Errorf("保存 state 失败: %w", err)
	}
	return nil
}

func Sync(cfg config.Config, st model.State) (model.State, SyncResult, error) {
	scanner := discovery.NewScanner(cfg.ExcludePatterns, time.Duration(cfg.Scan.RunningWindowMinutes)*time.Minute)
	runsFound, err := scanner.Discover(cfg.WatchedRoots)
	if err != nil {
		return st, SyncResult{}, err
	}
	st.Discovered = runsFound
	pruned := selection.PruneSelected(&st)
	res := SyncResult{Discovered: len(st.Discovered), Selected: len(st.Selected), Pruned: pruned}
	return st, res, nil
}

func ApplySelection(cfg config.Config, st model.State) (int, error) {
	selected := selection.SelectedRuns(st)
	return runs.ApplySelected(cfg.Managed.RunDir, selected, cfg.Managed.CleanupStale)
}
