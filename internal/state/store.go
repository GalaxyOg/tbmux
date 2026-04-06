package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tbmux/internal/model"
)

func New() model.State {
	return model.State{
		Version:    1,
		UpdatedAt:  time.Now().UTC(),
		Discovered: []model.RunRecord{},
		Selected:   map[string]model.SelectionEntry{},
	}
}

func Load(path string) (model.State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return model.State{}, err
	}
	var st model.State
	if err := json.Unmarshal(b, &st); err != nil {
		return model.State{}, fmt.Errorf("解析 state 失败: %w", err)
	}
	if st.Selected == nil {
		st.Selected = map[string]model.SelectionEntry{}
	}
	if st.Discovered == nil {
		st.Discovered = []model.RunRecord{}
	}
	if st.Version == 0 {
		st.Version = 1
	}
	return st, nil
}

func Save(path string, st model.State) error {
	st.UpdatedAt = time.Now().UTC()
	if st.Selected == nil {
		st.Selected = map[string]model.SelectionEntry{}
	}
	if st.Discovered == nil {
		st.Discovered = []model.RunRecord{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
