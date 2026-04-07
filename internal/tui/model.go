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
	"tbmux/internal/process"
	"tbmux/internal/selection"
	"tbmux/internal/tailscale"
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
	listTop         int
	searchMode      bool
	searchInput     string
	helpVisible     bool
	exitConfirmMode bool
	nameOffset      int

	width  int
	height int

	tbRunning bool
	tbPID     int
	tbErr     error

	tailscaleDetected bool
	tailscaleBinary   string
	tailscaleMethod   string
	tailscaleServeOn  bool
	tailscaleURL      string
	tailscaleErr      error
	tailscaleBusy     bool

	statusMsg   string
	statusLevel string
	lastErr     error
	quitting    bool
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

type processStatusMsg struct {
	running bool
	pid     int
	err     error
}

type tailscaleStatusMsg struct {
	detected bool
	binary   string
	method   string
	serveOn  bool
	url      string
	note     string
	err      error
}

type tailscaleActionMsg struct {
	enable bool
	out    string
	err    error
}

type autoServeSavedMsg struct {
	auto bool
	err  error
}

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	styleMeta  = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	styleInfo  = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	styleWarn  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleErr   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	styleOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Bold(true)
	styleFaint = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	styleCursor    = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	styleRowActive = lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("24"))
	stylePane      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("67")).Padding(0, 1)
)

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
		helpVisible:     true,
		statusMsg:       "就绪: j/k移动, ←/→查看长名称, space切换, g tailscale开关, m 自动开关, a应用",
		statusLevel:     "info",
		searchMode:      false,
		exitConfirmMode: false,
		width:           120,
		height:          32,
	}
	m.rebuildIndices()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshProcessCmd(), m.refreshTailscaleCmd(true))
}

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
			m.statusLevel = "error"
			return m, nil
		}
		m.dirty = false
		m.statusMsg = fmt.Sprintf("apply 成功: symlink=%d", msg.applied)
		m.statusLevel = "ok"
		return m, tea.Batch(m.refreshProcessCmd(), m.refreshTailscaleCmd(m.cfg.Tailscale.AutoServe))
	case syncedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.statusMsg = "sync 失败: " + msg.err.Error()
			m.statusLevel = "error"
			return m, nil
		}
		m.st = msg.st
		pruneUnknownSelected(&m.draftSelected, m.st)
		m.rebuildIndices()
		m.statusMsg = msg.msg
		m.statusLevel = "ok"
		return m, tea.Batch(m.refreshProcessCmd(), m.refreshTailscaleCmd(m.cfg.Tailscale.AutoServe))
	case processStatusMsg:
		m.tbRunning = msg.running
		m.tbPID = msg.pid
		m.tbErr = msg.err
		if msg.err != nil {
			m.lastErr = msg.err
		}
		return m, nil
	case tailscaleStatusMsg:
		m.tailscaleDetected = msg.detected
		m.tailscaleBinary = msg.binary
		m.tailscaleMethod = msg.method
		m.tailscaleServeOn = msg.serveOn
		m.tailscaleURL = msg.url
		m.tailscaleErr = msg.err
		m.tailscaleBusy = false
		if msg.err != nil {
			m.lastErr = msg.err
			m.statusMsg = "tailscale: " + msg.err.Error()
			m.statusLevel = "warn"
			return m, nil
		}
		if strings.TrimSpace(msg.note) != "" {
			m.statusMsg = "tailscale: " + msg.note
			m.statusLevel = "ok"
		}
		return m, nil
	case tailscaleActionMsg:
		m.tailscaleBusy = false
		if msg.err != nil {
			m.lastErr = msg.err
			m.statusMsg = "tailscale 操作失败: " + msg.err.Error()
			m.statusLevel = "error"
			return m, m.refreshTailscaleCmd(false)
		}
		if msg.enable {
			m.statusMsg = "tailscale serve 已请求开启"
		} else {
			m.statusMsg = "tailscale serve 已请求关闭"
		}
		if strings.TrimSpace(msg.out) != "" {
			m.statusMsg += " | " + firstLine(msg.out)
		}
		m.statusLevel = "ok"
		return m, m.refreshTailscaleCmd(false)
	case autoServeSavedMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			m.statusMsg = "保存 auto_serve 失败: " + msg.err.Error()
			m.statusLevel = "error"
			return m, nil
		}
		if msg.auto {
			m.statusMsg = "tailscale.auto_serve 已开启"
			m.statusLevel = "ok"
			return m, m.refreshTailscaleCmd(true)
		}
		m.statusMsg = "tailscale.auto_serve 已关闭"
		m.statusLevel = "warn"
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureCursorVisible()
		m.clampNameOffset()
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.search = strings.TrimSpace(m.searchInput)
		m.filter.Match = m.search
		m.searchMode = false
		m.rebuildIndices()
		m.statusMsg = "搜索已应用"
		m.statusLevel = "ok"
		return m, nil
	case "esc":
		m.searchMode = false
		m.statusMsg = "取消搜索输入"
		m.statusLevel = "info"
		return m, nil
	case "backspace":
		if len(m.searchInput) > 0 {
			m.searchInput = m.searchInput[:len(m.searchInput)-1]
		}
		return m, nil
	default:
		s := msg.String()
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
			m.statusLevel = "warn"
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
		m.nameOffset = 0
		m.ensureCursorVisible()
		m.exitConfirmMode = false
		return m, nil
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		m.nameOffset = 0
		m.ensureCursorVisible()
		m.exitConfirmMode = false
		return m, nil
	case "left":
		m.shiftNameOffset(-8)
		m.exitConfirmMode = false
		return m, nil
	case "right":
		m.shiftNameOffset(8)
		m.exitConfirmMode = false
		return m, nil
	case " ":
		m.toggleCurrent()
		m.exitConfirmMode = false
		return m, nil
	case "x":
		m.clearAllDraftSelected()
		m.exitConfirmMode = false
		return m, nil
	case "/":
		m.searchMode = true
		m.searchInput = m.search
		m.statusMsg = "输入搜索关键词，回车应用"
		m.statusLevel = "info"
		m.exitConfirmMode = false
		return m, nil
	case "c":
		m.filter = model.Filter{}
		m.search = ""
		m.searchInput = ""
		m.rebuildIndices()
		m.statusMsg = "已清空筛选"
		m.statusLevel = "ok"
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
		m.statusLevel = "ok"
		m.exitConfirmMode = false
		return m, nil
	case "a":
		m.exitConfirmMode = false
		return m, m.applyCmd()
	case "s":
		m.exitConfirmMode = false
		return m, m.syncCmd()
	case "g":
		if m.tailscaleBusy {
			m.statusMsg = "tailscale 操作进行中"
			m.statusLevel = "warn"
			return m, nil
		}
		m.tailscaleBusy = true
		m.statusMsg = "正在执行 tailscale serve 切换..."
		m.statusLevel = "info"
		m.exitConfirmMode = false
		return m, m.tailscaleToggleCmd(!m.tailscaleServeOn)
	case "m":
		m.cfg.Tailscale.AutoServe = !m.cfg.Tailscale.AutoServe
		m.exitConfirmMode = false
		return m, m.saveAutoServeCmd(m.cfg.Tailscale.AutoServe)
	default:
		return m, nil
	}
}

func (m Model) View() string {
	if m.quitting {
		return "已退出 tbmux tui\n"
	}

	leftWidth, rightWidth := m.paneWidths()
	title := styleTitle.Render("tbmux tui")
	meta := styleMeta.Render(fmt.Sprintf("config=%s | discovered=%d | selected(draft)=%d | dirty=%t", m.cfgPath, len(m.st.Discovered), len(m.draftSelected), m.dirty))
	filterLine := styleMeta.Render("filter=" + filterSummary(m.filter))

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.renderListPane(leftWidth), " ", m.renderDetailPane(rightWidth))

	var lines []string
	lines = append(lines, title, meta, filterLine)
	if m.searchMode {
		lines = append(lines, styleInfo.Render("search> "+m.searchInput))
	}
	lines = append(lines, body)
	if m.helpVisible {
		lines = append(lines, helpText())
	}
	lines = append(lines, m.renderStatus())
	return strings.Join(lines, "\n") + "\n"
}

func (m Model) renderStatus() string {
	extra := ""
	if m.tailscaleBusy {
		extra = " [tailscale busy]"
	}
	msg := m.statusMsg + extra
	switch m.statusLevel {
	case "ok":
		return styleOK.Render(msg)
	case "warn":
		return styleWarn.Render(msg)
	case "error":
		return styleErr.Render(msg)
	default:
		return styleInfo.Render(msg)
	}
}

func (m *Model) toggleCurrent() {
	run, ok := m.currentRun()
	if !ok {
		m.statusMsg = "当前无可操作条目"
		m.statusLevel = "warn"
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
	m.statusLevel = "ok"
}

func (m *Model) clearAllDraftSelected() {
	if len(m.draftSelected) == 0 {
		m.statusMsg = "draft selected 已为空"
		m.statusLevel = "info"
		return
	}
	m.draftSelected = map[string]model.SelectionEntry{}
	m.dirty = true
	m.statusMsg = "已清空全部 draft selected（按 a 应用）"
	m.statusLevel = "ok"
}

func (m *Model) toggleRunningFilter() {
	if m.filter.RunningOnly == nil {
		v := true
		m.filter.RunningOnly = &v
		m.statusMsg = "running 筛选: only running"
		m.statusLevel = "ok"
		return
	}
	if *m.filter.RunningOnly {
		v := false
		m.filter.RunningOnly = &v
		m.statusMsg = "running 筛选: only not-running"
		m.statusLevel = "ok"
		return
	}
	m.filter.RunningOnly = nil
	m.statusMsg = "running 筛选: all"
	m.statusLevel = "ok"
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
		m.listTop = 0
		m.nameOffset = 0
		return
	}
	if m.cursor >= len(m.indices) {
		m.cursor = len(m.indices) - 1
	}
	m.clampNameOffset()
	m.ensureCursorVisible()
}

func (m *Model) ensureCursorVisible() {
	visibleRows := m.listRowsCapacity()
	if visibleRows < 1 {
		visibleRows = 1
	}
	if m.cursor < m.listTop {
		m.listTop = m.cursor
	}
	if m.cursor >= m.listTop+visibleRows {
		m.listTop = m.cursor - visibleRows + 1
	}
	if m.listTop < 0 {
		m.listTop = 0
	}
	maxTop := len(m.indices) - visibleRows
	if maxTop < 0 {
		maxTop = 0
	}
	if m.listTop > maxTop {
		m.listTop = maxTop
	}
}

func (m Model) listRowsCapacity() int {
	if m.height <= 0 {
		return 16
	}
	base := 8 // 标题/状态/帮助等
	if m.searchMode {
		base++
	}
	if m.helpVisible {
		base++
	}
	rows := m.height - base
	if rows < 6 {
		rows = 6
	}
	return rows
}

func (m Model) currentRun() (model.RunRecord, bool) {
	if len(m.indices) == 0 || m.cursor < 0 || m.cursor >= len(m.indices) {
		return model.RunRecord{}, false
	}
	return m.st.Discovered[m.indices[m.cursor]], true
}

func (m Model) paneWidths() (int, int) {
	w := m.width
	if w <= 0 {
		w = 120
	}
	if w < 72 {
		w = 72
	}
	left := int(float64(w) * 0.56)
	if left < 34 {
		left = 34
	}
	right := w - left - 1
	if right < 24 {
		right = 24
		left = w - right - 1
		if left < 30 {
			left = 30
			right = w - left - 1
		}
	}
	return left, right
}

func (m Model) listNameWidth() int {
	left, _ := m.paneWidths()
	content := max(16, left-4)
	w := content - 11 // cursor + selected + run + spaces
	if w < 8 {
		w = 8
	}
	return w
}

func (m *Model) clampNameOffset() {
	r, ok := m.currentRun()
	if !ok {
		m.nameOffset = 0
		return
	}
	_, off, maxOff := windowText(r.Name, m.nameOffset, m.listNameWidth())
	if maxOff == 0 {
		m.nameOffset = 0
		return
	}
	m.nameOffset = off
}

func (m *Model) shiftNameOffset(delta int) {
	r, ok := m.currentRun()
	if !ok {
		m.statusMsg = "当前无可操作条目"
		m.statusLevel = "warn"
		return
	}
	_, off, maxOff := windowText(r.Name, m.nameOffset+delta, m.listNameWidth())
	m.nameOffset = off
	if maxOff == 0 {
		m.statusMsg = "当前名称无需水平滚动"
		m.statusLevel = "info"
		return
	}
	m.statusMsg = fmt.Sprintf("名称窗口偏移: %d/%d", m.nameOffset, maxOff)
	m.statusLevel = "info"
}

func (m Model) renderListPane(width int) string {
	contentWidth := max(12, width-4)
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75")).Render("Discovered")
	nameHint := styleFaint.Render("(←/→ 查看完整名称)")
	lines := []string{clipRunes(header+" "+nameHint, contentWidth)}
	if len(m.indices) == 0 {
		lines = append(lines, styleFaint.Render("(no runs matched)"))
		return stylePane.Width(width).Render(strings.Join(lines, "\n"))
	}
	cap := m.listRowsCapacity()
	start := m.listTop
	if start < 0 {
		start = 0
	}
	end := start + cap
	if end > len(m.indices) {
		end = len(m.indices)
	}
	nameWidth := m.listNameWidth()
	for i := start; i < end; i++ {
		r := m.st.Discovered[m.indices[i]]
		cursor := " "
		if i == m.cursor {
			cursor = styleCursor.Render(">")
		}
		sel := styleFaint.Render("[ ]")
		if _, ok := m.draftSelected[r.ID]; ok {
			sel = styleOK.Render("[x]")
		}
		runState := styleFaint.Render("IDLE")
		if r.IsRunning {
			runState = styleOK.Render("RUN")
		}
		offset := 0
		if i == m.cursor {
			offset = m.nameOffset
		}
		namePart, _, _ := windowText(r.Name, offset, nameWidth)
		line := fmt.Sprintf("%s %s %s %s", cursor, sel, runState, clipRunes(namePart, nameWidth))
		if i == m.cursor {
			line = styleRowActive.Render(line)
		}
		lines = append(lines, line)
	}
	if end < len(m.indices) {
		lines = append(lines, styleFaint.Render(fmt.Sprintf("... %d more", len(m.indices)-end)))
	}
	return stylePane.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderDetailPane(width int) string {
	contentWidth := max(14, width-4)
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("150")).Render("Detail")
	lines := []string{clipRunes(header, contentWidth)}
	r, ok := m.currentRun()
	if !ok {
		lines = append(lines, styleFaint.Render("(none)"))
		return stylePane.Width(width).Render(strings.Join(lines, "\n"))
	}
	selState := styleWarn.Render("NO")
	if _, ok := m.draftSelected[r.ID]; ok {
		selState = styleOK.Render("YES")
	}
	runningState := styleWarn.Render("IDLE")
	if r.IsRunning {
		runningState = styleOK.Render("RUNNING")
	}
	lines = append(lines,
		clipRunes("id: "+r.ID, contentWidth),
		clipRunes("name: "+r.Name, contentWidth),
		"selected(draft): "+selState,
		"running: "+runningState,
		clipRunes("updated: "+r.LastUpdatedAt.Format(time.RFC3339), contentWidth),
		clipRunes("watch_root: "+r.WatchRoot, contentWidth),
		clipRunes("source: "+r.SourcePath, contentWidth),
	)

	tbState := styleWarn.Render("STOPPED")
	if m.tbRunning {
		tbState = styleOK.Render("RUNNING")
	}
	tbURL := fmt.Sprintf("http://%s:%d", m.cfg.TensorBoard.Host, m.cfg.TensorBoard.Port)
	lines = append(lines,
		"",
		clipRunes("TensorBoard", contentWidth),
		"status: "+tbState,
		clipRunes(fmt.Sprintf("pid: %d", m.tbPID), contentWidth),
		clipRunes("url: "+tbURL, contentWidth),
	)
	if m.tbErr != nil {
		lines = append(lines, styleWarn.Render(clipRunes("process_err: "+m.tbErr.Error(), contentWidth)))
	}

	autoState := styleWarn.Render("OFF")
	if m.cfg.Tailscale.AutoServe {
		autoState = styleOK.Render("ON")
	}
	serveState := styleWarn.Render("OFF")
	if m.tailscaleServeOn {
		serveState = styleOK.Render("ON")
	}
	lines = append(lines,
		"",
		clipRunes("Tailscale", contentWidth),
		"auto_serve: "+autoState,
	)
	if !m.tailscaleDetected {
		lines = append(lines, styleWarn.Render(clipRunes("binary: not found", contentWidth)))
	} else {
		lines = append(lines,
			clipRunes("binary: "+m.tailscaleBinary, contentWidth),
			clipRunes("method: "+m.tailscaleMethod, contentWidth),
			"serve: "+serveState,
		)
		if m.tailscaleURL != "" {
			lines = append(lines, clipRunes("tailnet: "+m.tailscaleURL, contentWidth))
		} else {
			lines = append(lines, styleFaint.Render(clipRunes("tailnet: (not available)", contentWidth)))
		}
	}
	if m.tailscaleErr != nil {
		lines = append(lines, styleWarn.Render(clipRunes("tailscale_err: "+m.tailscaleErr.Error(), contentWidth)))
	}
	lines = append(lines, styleFaint.Render(clipRunes("keys: g toggle serve | m toggle auto", contentWidth)))

	return stylePane.Width(width).Render(strings.Join(lines, "\n"))
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

func (m Model) refreshProcessCmd() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		running, pid, err := process.Status(cfg.Managed.PidPath)
		return processStatusMsg{running: running, pid: pid, err: err}
	}
}

func (m Model) refreshTailscaleCmd(autoEnsure bool) tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		det, err := tailscale.Detect(cfg.Tailscale.Binary)
		if err != nil {
			return tailscaleStatusMsg{detected: false, err: err}
		}
		msg := tailscaleStatusMsg{
			detected: true,
			binary:   det.Path,
			method:   det.Method,
		}
		statusOut, statusErr := tailscale.ServeStatus(det.Path)
		if statusErr == nil {
			msg.url = tailscale.ParseServeURL(statusOut)
			msg.serveOn = msg.url != ""
		}
		if autoEnsure && cfg.Tailscale.AutoServe && !msg.serveOn {
			serveOut, serveErr := tailscale.RunServe(det.Path, cfg.Tailscale.ServeURL)
			if strings.TrimSpace(serveOut) != "" {
				msg.note = firstLine(serveOut)
			}
			if serveErr != nil {
				msg.err = serveErr
				return msg
			}
			statusOut, statusErr = tailscale.ServeStatus(det.Path)
			if statusErr == nil {
				msg.url = tailscale.ParseServeURL(statusOut)
				msg.serveOn = msg.url != ""
			}
			if msg.url != "" {
				msg.note = "已自动开启 tailscale serve"
			}
		}
		if statusErr != nil && msg.err == nil {
			msg.err = statusErr
		}
		return msg
	}
}

func (m Model) tailscaleToggleCmd(enable bool) tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		det, err := tailscale.Detect(cfg.Tailscale.Binary)
		if err != nil {
			return tailscaleActionMsg{enable: enable, err: err}
		}
		if enable {
			out, runErr := tailscale.RunServe(det.Path, cfg.Tailscale.ServeURL)
			return tailscaleActionMsg{enable: enable, out: out, err: runErr}
		}
		out, runErr := tailscale.RunServeOff(det.Path)
		return tailscaleActionMsg{enable: enable, out: out, err: runErr}
	}
}

func (m Model) saveAutoServeCmd(auto bool) tea.Cmd {
	cfg := m.cfg
	cfg.Tailscale.AutoServe = auto
	return func() tea.Msg {
		err := config.Save(m.cfgPath, cfg)
		return autoServeSavedMsg{auto: auto, err: err}
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
	return styleFaint.Render("keys: j/k or ↑/↓ move | ←/→ name-scroll | space toggle | x clear selected | / search | r running | t today | c clear filter | s sync | a apply | g tailscale on/off | m auto_serve on/off | ? help | q quit")
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clipRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func windowText(s string, offset, width int) (string, int, int) {
	if width <= 0 {
		return "", 0, 0
	}
	r := []rune(s)
	if len(r) <= width {
		return s, 0, 0
	}
	if width < 3 {
		if offset < 0 {
			offset = 0
		}
		if offset >= len(r) {
			offset = len(r) - 1
		}
		return string(r[offset]), offset, len(r) - 1
	}

	segmentWidth := width - 2
	maxOffset := len(r) - segmentWidth
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	end := offset + segmentWidth
	if end > len(r) {
		end = len(r)
	}
	leftMark := " "
	rightMark := " "
	if offset > 0 {
		leftMark = "<"
	}
	if end < len(r) {
		rightMark = ">"
	}
	return leftMark + string(r[offset:end]) + rightMark, offset, maxOffset
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "\n")
	return strings.TrimSpace(parts[0])
}
