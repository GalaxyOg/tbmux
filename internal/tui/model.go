package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tbmux/internal/app"
	"tbmux/internal/config"
	"tbmux/internal/model"
	"tbmux/internal/selection"
)

type Model struct {
	cfgPath string
	cfg     config.Config
	st      model.State

	filter model.Filter
	search string

	draftSelected map[string]model.SelectionEntry
	dirty         bool

	cursor          int
	indices         []int
	searchMode      bool
	searchInput     string
	helpVisible     bool
	exitConfirmMode bool

	statusMsg string
	lastErr   error
	quitting  bool
}

type appliedMsg struct {
	applied int
	err     error
}

type syncedMsg struct {
	st  model.State
	msg string
	err error
}

func New(cfgPath string, cfg config.Config, st model.State, f model.Filter) Model {
	draft := make(map[string]model.SelectionEntry, len(st.Selected))
	for k, v := range st.Selected {
		draft[k] = v
	}
	m := Model{
		cfgPath:         cfgPath,
		cfg:             cfg,
		st:              st,
		filter:          f,
		draftSelected:   draft,
		indices:         nil,
		helpVisible:     true,
		statusMsg:       "就绪: j/k移动, space切换, /搜索, a应用, q退出",
		searchMode:      false,
		exitConfirmMode: false,
	}
	m.rebuildIndices()
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.searchMode {
			return m.updateSearch(msg)
		}
		return m.updateNormal(msg)
	case appliedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.statusMsg = "apply 失败: " + msg.err.Error()
			return m, nil
		}
		m.dirty = false
		m.statusMsg = fmt.Sprintf("apply 成功: symlink=%d", msg.applied)
		return m, nil
	case syncedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.statusMsg = "sync 失败: " + msg.err.Error()
			return m, nil
		}
		m.st = msg.st
		pruneUnknownSelected(&m.draftSelected, m.st)
		m.rebuildIndices()
		m.statusMsg = msg.msg
		return m, nil
	case tea.WindowSizeMsg:
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "enter":
		m.search = strings.TrimSpace(m.searchInput)
		m.filter.Match = m.search
		m.searchMode = false
		m.rebuildIndices()
		m.statusMsg = "搜索已应用"
		return m, nil
	case "esc":
		m.searchMode = false
		m.statusMsg = "取消搜索输入"
		return m, nil
	case "backspace":
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
		return m, nil
	default:
		if len(s) == 1 {
			m.searchInput += s
		}
		return m, nil
	}
}

func (m Model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.dirty && !m.exitConfirmMode {
			m.exitConfirmMode = true
			m.statusMsg = "有未 apply 变更，再按 q 放弃并退出"
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.helpVisible = !m.helpVisible
		return m, nil
	case "down", "j":
		if m.cursor < len(m.indices)-1 {
			m.cursor++
		}
		m.exitConfirmMode = false
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.exitConfirmMode = false
		return m, nil
	case " ":
		m.toggleCurrent()
		m.exitConfirmMode = false
		return m, nil
	case "/":
		m.searchMode = true
		m.searchInput = m.search
		m.statusMsg = "输入搜索关键词，回车应用"
		m.exitConfirmMode = false
		return m, nil
	case "c":
		m.filter = model.Filter{}
		m.search = ""
		m.searchInput = ""
		m.rebuildIndices()
		m.statusMsg = "已清空筛选"
		m.exitConfirmMode = false
		return m, nil
	case "r":
		m.toggleRunningFilter()
		m.rebuildIndices()
		m.exitConfirmMode = false
		return m, nil
	case "t":
		m.filter.Today = !m.filter.Today
		m.rebuildIndices()
		m.statusMsg = "today 筛选已切换"
		m.exitConfirmMode = false
		return m, nil
	case "a":
		m.exitConfirmMode = false
		return m, m.applyCmd()
	case "s":
		m.exitConfirmMode = false
		return m, m.syncCmd()
	default:
		return m, nil
	}
}

func (m Model) View() string {
	if m.quitting {
		return "已退出 tbmux tui\n"
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33")).Render("tbmux tui")
	left := m.renderListPane(80)
	right := m.renderDetailPane(80)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "\n", right)

	footer := m.statusMsg
	if m.lastErr != nil {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(footer)
	}

	var lines []string
	lines = append(lines, title)
	lines = append(lines, fmt.Sprintf("config=%s | discovered=%d | selected(draft)=%d | dirty=%t", m.cfgPath, len(m.st.Discovered), len(m.draftSelected), m.dirty))
	lines = append(lines, fmt.Sprintf("filter=%s", filterSummary(m.filter)))
	if m.searchMode {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("search> "+m.searchInput))
	}
	lines = append(lines, body)
	if m.helpVisible {
		lines = append(lines, helpText())
	}
	lines = append(lines, footer)

	return strings.Join(lines, "\n") + "\n"
}

func (m *Model) toggleCurrent() {
	run, ok := m.currentRun()
	if !ok {
		m.statusMsg = "当前无可操作条目"
		return
	}
	if _, exists := m.draftSelected[run.ID]; exists {
		delete(m.draftSelected, run.ID)
		m.statusMsg = "已取消选择: " + run.Name
	} else {
		m.draftSelected[run.ID] = model.SelectionEntry{Source: "tui_manual", SelectedAt: time.Now().UTC()}
		m.statusMsg = "已选择: " + run.Name
	}
	m.dirty = true
}

func (m *Model) toggleRunningFilter() {
	if m.filter.RunningOnly == nil {
		v := true
		m.filter.RunningOnly = &v
		m.statusMsg = "running 筛选: only running"
		return
	}
	if *m.filter.RunningOnly {
		v := false
		m.filter.RunningOnly = &v
		m.statusMsg = "running 筛选: only not-running"
		return
	}
	m.filter.RunningOnly = nil
	m.statusMsg = "running 筛选: all"
}

func (m *Model) rebuildIndices() {
	filtered := selection.ApplyFilter(m.st.Discovered, m.filter, time.Now())
	idSet := make(map[string]struct{}, len(filtered))
	for _, r := range filtered {
		idSet[r.ID] = struct{}{}
	}
	idx := make([]int, 0, len(filtered))
	for i := range m.st.Discovered {
		if _, ok := idSet[m.st.Discovered[i].ID]; ok {
			idx = append(idx, i)
		}
	}
	sort.Ints(idx)
	m.indices = idx
	if len(m.indices) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.indices) {
		m.cursor = len(m.indices) - 1
	}
}

func (m Model) currentRun() (model.RunRecord, bool) {
	if len(m.indices) == 0 || m.cursor < 0 || m.cursor >= len(m.indices) {
		return model.RunRecord{}, false
	}
	return m.st.Discovered[m.indices[m.cursor]], true
}

func (m Model) renderListPane(width int) string {
	header := lipgloss.NewStyle().Bold(true).Render("Discovered")
	lines := []string{header}
	if len(m.indices) == 0 {
		lines = append(lines, "(no runs matched)")
		return strings.Join(lines, "\n")
	}
	for i, idx := range m.indices {
		r := m.st.Discovered[idx]
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		sel := "[ ]"
		if _, ok := m.draftSelected[r.ID]; ok {
			sel = "[x]"
		}
		running := "idle"
		if r.IsRunning {
			running = "run"
		}
		line := fmt.Sprintf("%s %s %-20s %-4s %s", cursor, sel, trim(r.Name, 20), running, r.LastUpdatedAt.Format("01-02 15:04"))
		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(line)
		}
		lines = append(lines, line)
		if len(lines) >= 25 {
			break
		}
	}
	return lipgloss.NewStyle().Width(width).Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
}

func (m Model) renderDetailPane(width int) string {
	header := lipgloss.NewStyle().Bold(true).Render("Detail")
	lines := []string{header}
	r, ok := m.currentRun()
	if !ok {
		lines = append(lines, "(none)")
		return lipgloss.NewStyle().Width(width).Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
	}
	selState := "no"
	if _, ok := m.draftSelected[r.ID]; ok {
		selState = "yes"
	}
	lines = append(lines,
		"id: "+r.ID,
		"name: "+r.Name,
		"selected(draft): "+selState,
		fmt.Sprintf("running: %t", r.IsRunning),
		"updated: "+r.LastUpdatedAt.Format(time.RFC3339),
		"watch_root: "+r.WatchRoot,
		"source: "+r.SourcePath,
	)
	return lipgloss.NewStyle().Width(width).Border(lipgloss.RoundedBorder()).Padding(0, 1).Render(strings.Join(lines, "\n"))
}

func (m Model) applyCmd() tea.Cmd {
	return func() tea.Msg {
		m.st.Selected = make(map[string]model.SelectionEntry, len(m.draftSelected))
		for k, v := range m.draftSelected {
			m.st.Selected[k] = v
		}
		if err := app.SaveState(m.cfg, m.st); err != nil {
			return appliedMsg{err: err}
		}
		n, err := app.ApplySelection(m.cfg, m.st)
		return appliedMsg{applied: n, err: err}
	}
}

func (m Model) syncCmd() tea.Cmd {
	return func() tea.Msg {
		st, res, err := app.Sync(m.cfg, m.st)
		if err != nil {
			return syncedMsg{err: err}
		}
		if err := app.SaveState(m.cfg, st); err != nil {
			return syncedMsg{err: err}
		}
		return syncedMsg{st: st, msg: fmt.Sprintf("sync 完成: discovered=%d selected=%d pruned=%d", res.Discovered, res.Selected, res.Pruned)}
	}
}

func pruneUnknownSelected(sel *map[string]model.SelectionEntry, st model.State) {
	known := make(map[string]struct{}, len(st.Discovered))
	for _, r := range st.Discovered {
		known[r.ID] = struct{}{}
	}
	for k := range *sel {
		if _, ok := known[k]; !ok {
			delete(*sel, k)
		}
	}
}

func helpText() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(
		"keys: j/k or ↑/↓ move | space toggle | / search | r running | t today | c clear filter | s sync | a apply | ? help | q quit",
	)
}

func filterSummary(f model.Filter) string {
	parts := make([]string, 0)
	if f.Today {
		parts = append(parts, "today")
	}
	if f.Hours > 0 {
		parts = append(parts, fmt.Sprintf("hours=%d", f.Hours))
	}
	if f.Days > 0 {
		parts = append(parts, fmt.Sprintf("days=%d", f.Days))
	}
	if f.RunningOnly != nil {
		if *f.RunningOnly {
			parts = append(parts, "running")
		} else {
			parts = append(parts, "not-running")
		}
	}
	if f.Under != "" {
		parts = append(parts, "under="+f.Under)
	}
	if f.Match != "" {
		parts = append(parts, "match="+f.Match)
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, ",")
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
