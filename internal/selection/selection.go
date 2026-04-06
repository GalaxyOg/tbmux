package selection

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tbmux/internal/model"
)

func PruneSelected(st *model.State) int {
	if st.Selected == nil {
		st.Selected = map[string]model.SelectionEntry{}
	}
	known := map[string]struct{}{}
	for _, run := range st.Discovered {
		known[run.ID] = struct{}{}
	}
	removed := 0
	for id := range st.Selected {
		if _, ok := known[id]; !ok {
			delete(st.Selected, id)
			removed++
		}
	}
	return removed
}

func AddByTokens(st *model.State, tokens []string, source string) (int, error) {
	if len(tokens) == 0 {
		return 0, errors.New("未提供 run id 或名称")
	}
	added := 0
	for _, tok := range tokens {
		run, err := resolveOne(st.Discovered, tok)
		if err != nil {
			return added, err
		}
		if st.Selected == nil {
			st.Selected = map[string]model.SelectionEntry{}
		}
		if _, exists := st.Selected[run.ID]; exists {
			continue
		}
		st.Selected[run.ID] = model.SelectionEntry{Source: source, SelectedAt: time.Now().UTC()}
		added++
	}
	return added, nil
}

func RemoveByTokens(st *model.State, tokens []string) (int, error) {
	if len(tokens) == 0 {
		return 0, errors.New("未提供 run id 或名称")
	}
	removed := 0
	for _, tok := range tokens {
		run, err := resolveOne(st.Discovered, tok)
		if err != nil {
			return removed, err
		}
		if _, ok := st.Selected[run.ID]; ok {
			delete(st.Selected, run.ID)
			removed++
		}
	}
	return removed, nil
}

func Clear(st *model.State) {
	st.Selected = map[string]model.SelectionEntry{}
}

func SelectByFilter(st *model.State, f model.Filter, mode string) int {
	targets := ApplyFilter(st.Discovered, f, time.Now())
	if st.Selected == nil {
		st.Selected = map[string]model.SelectionEntry{}
	}
	n := 0
	switch mode {
	case "set":
		newSel := map[string]model.SelectionEntry{}
		for _, run := range targets {
			newSel[run.ID] = model.SelectionEntry{Source: "rule", SelectedAt: time.Now().UTC()}
		}
		st.Selected = newSel
		n = len(targets)
	case "remove":
		for _, run := range targets {
			if _, ok := st.Selected[run.ID]; ok {
				delete(st.Selected, run.ID)
				n++
			}
		}
	default:
		for _, run := range targets {
			if _, ok := st.Selected[run.ID]; ok {
				continue
			}
			st.Selected[run.ID] = model.SelectionEntry{Source: "rule", SelectedAt: time.Now().UTC()}
			n++
		}
	}
	return n
}

func SelectedRuns(st model.State) []model.RunRecord {
	out := make([]model.RunRecord, 0)
	for _, run := range st.Discovered {
		if _, ok := st.Selected[run.ID]; ok {
			out = append(out, run)
		}
	}
	return out
}

func resolveOne(runs []model.RunRecord, token string) (model.RunRecord, error) {
	tok := strings.TrimSpace(token)
	if tok == "" {
		return model.RunRecord{}, errors.New("空 token")
	}
	for _, run := range runs {
		if run.ID == tok {
			return run, nil
		}
	}
	for _, run := range runs {
		if run.Name == tok {
			return run, nil
		}
	}
	cands := make([]model.RunRecord, 0)
	ltok := strings.ToLower(tok)
	for _, run := range runs {
		if strings.Contains(strings.ToLower(run.Name), ltok) {
			cands = append(cands, run)
		}
	}
	if len(cands) == 1 {
		return cands[0], nil
	}
	if len(cands) > 1 {
		return model.RunRecord{}, fmt.Errorf("%q 匹配到多个 run，请使用 id 或精确名称", token)
	}
	return model.RunRecord{}, fmt.Errorf("未找到 run: %s", token)
}
