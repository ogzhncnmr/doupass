package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/ogzhncnmr/doupass/internal/codex"
	"github.com/ogzhncnmr/doupass/internal/opencode"
	"github.com/ogzhncnmr/doupass/internal/policy"
	"github.com/spf13/cobra"
)

type landingChoice struct {
	Label       string
	Description string
	Args        []string
	Quit        bool
}

var landingChoices = []landingChoice{
	{Label: "Show status", Description: "Policy, audit log, and integrations at a glance", Args: []string{"doctor"}},
	{Label: "Preview setup", Description: "See what would be wired, touch nothing", Args: []string{"setup", "--dry-run"}},
	{Label: "Apply setup", Description: "Wire the tools installed on this machine", Args: []string{"setup"}},
	{Label: "Create a policy", Description: "Pick or replace a ready-made doupass.yml, zero writing", Args: nil},
	{Label: "Recent agent activity", Description: "Last 20 audit entries", Args: []string{"log", "tail"}},
	{Label: "Verify audit log", Description: "Check the hash chain for tampering", Args: []string{"log", "verify"}},
	{Label: "Full help", Description: "Every command and flag", Args: []string{"help"}},
	{Label: "Quit", Quit: true},
}

var landingIcons = map[string]string{
	"Show status":           "●",
	"Preview setup":         "◐",
	"Apply setup":           "✚",
	"Create a policy":       "✎",
	"Recent agent activity": "↻",
	"Verify audit log":      "✔",
	"Full help":             "≡",
	"Quit":                  "×",
}

type landingScreen int

const (
	screenMenu landingScreen = iota
	screenPreset
	screenRunning
	screenOutput
)

type presetOption struct {
	Name        string
	Description string
}

var presetOptions = []presetOption{
	{"starter", "block credential files, ask before risky commands, normal work stays allowed"},
	{"minimal", "only the highest-value credential denies, everything else allowed"},
	{"locked-down", "deny by default, explicitly allow what you trust"},
	{"red-team", "permissive but logs exfiltration patterns for review"},
}

var presetRuleCounts = buildPresetRuleCounts()

func buildPresetRuleCounts() map[string]int {
	counts := make(map[string]int, len(presetNames))
	for _, name := range presetNames {
		counts[name] = 0
		data, err := presetsFS.ReadFile("presets/" + name + ".yml")
		if err != nil {
			continue
		}
		p, err := policy.Parse(data)
		if err != nil {
			continue
		}
		counts[name] = len(p.Rules)
	}
	return counts
}

// matchingPreset reports which embedded preset a policy file is byte-identical
// to (CRLF-normalized), or "" when the policy was edited or written by hand.
func matchingPreset(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	norm := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	for _, name := range presetNames {
		data, err := presetsFS.ReadFile("presets/" + name + ".yml")
		if err != nil {
			continue
		}
		if bytes.Equal(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), norm) {
			return name
		}
	}
	return ""
}

type landingInfo struct {
	Version      string
	PolicyPath   string
	PolicyName   string
	PresetName   string
	Rules        int
	LintIssues   int
	Found        bool
	Invalid      bool
	AuditPath    string
	AuditExists  bool
	AuditOK      bool
	AuditEntries int
	Integrations int
}

type landingModel struct {
	choices []landingChoice
	cursor  int
	info    landingInfo
	width   int
	height  int

	screen         landingScreen
	presetIdx      int
	confirmReplace bool
	runTitle       string
	runID          int
	spinner        int
	output         []string
	runErr         string
	scroll         int
}

func newLandingModel() landingModel {
	return landingModel{choices: landingChoices, info: landingInfoData()}
}

func (m landingModel) Init() tea.Cmd { return nil }

type spinnerMsg struct{}
type runFinishedMsg struct {
	id     int
	output string
	errTxt string
}

func spinnerTick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinnerMsg{} })
}

// runLandingCommand executes a doupass command in the background while the
// menu keeps rendering, and delivers everything it printed as one message.
func runLandingCommand(id int, args []string) tea.Cmd {
	return func() tea.Msg {
		var out, errOut bytes.Buffer
		execErr := Execute(args, strings.NewReader(""), &out, &errOut)
		text := strings.TrimRight(out.String(), "\n")
		errTxt := ""
		if execErr != nil {
			if s := strings.TrimSpace(errOut.String()); s != "" {
				text += "\n\n" + s
			}
			errTxt = execErr.Error()
		}
		return runFinishedMsg{id: id, output: text, errTxt: errTxt}
	}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m landingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case spinnerMsg:
		if m.screen == screenRunning {
			m.spinner++
			return m, spinnerTick()
		}
	case runFinishedMsg:
		if msg.id != m.runID {
			return m, nil
		}
		m.output = splitLines(msg.output)
		m.runErr = msg.errTxt
		m.scroll = 0
		m.screen = screenOutput
		m.confirmReplace = false
		m.info = landingInfoData()
		return m, nil
	case tea.KeyMsg:
		switch m.screen {
		case screenMenu:
			return m.updateMenu(msg)
		case screenPreset:
			return m.updatePreset(msg)
		case screenRunning:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			}
		case screenOutput:
			switch msg.String() {
			case "enter", "esc", "q":
				m.screen = screenMenu
				return m, nil
			case "up", "k":
				m.scrollBy(-1)
			case "down", "j":
				m.scrollBy(1)
			case "pgup":
				m.scrollBy(-m.outputWindow() + 1)
			case "pgdown":
				m.scrollBy(m.outputWindow() - 1)
			case "home":
				m.scroll = 0
			case "end":
				m.scroll = 1 << 30
				m.clampScroll()
			case "ctrl+c":
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m landingModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.cursor = (m.cursor + len(m.choices) - 1) % len(m.choices)
	case "down", "j":
		m.cursor = (m.cursor + 1) % len(m.choices)
	case "enter":
		choice := m.choices[m.cursor]
		if choice.Quit {
			return m, tea.Quit
		}
		if choice.Label == "Create a policy" {
			m.screen = screenPreset
			m.presetIdx = 0
			m.confirmReplace = false
			return m, nil
		}
		return m.startRun(choice.Args)
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m landingModel) updatePreset(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.confirmReplace = false
		m.presetIdx = (m.presetIdx + len(presetOptions) - 1) % len(presetOptions)
	case "down", "j":
		m.confirmReplace = false
		m.presetIdx = (m.presetIdx + 1) % len(presetOptions)
	case "enter":
		return m.runPreset()
	case "esc", "q":
		if m.confirmReplace {
			m.confirmReplace = false
			return m, nil
		}
		m.screen = screenMenu
	default:
		if n := digitPressed(msg); n >= 1 && n <= len(presetOptions) {
			m.confirmReplace = false
			m.presetIdx = n - 1
		}
	}
	return m, nil
}

func digitPressed(msg tea.KeyMsg) int {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return 0
	}
	r := msg.Runes[0]
	if r < '1' || r > '9' {
		return 0
	}
	return int(r - '0')
}

// presetTargetPath is the file the selected preset would write: the existing
// policy when there is one, ./doupass.yml otherwise.
func (m landingModel) presetTargetPath() string {
	if m.info.Found && m.info.PolicyPath != "" {
		return m.info.PolicyPath
	}
	return "./doupass.yml"
}

func (m landingModel) presetArgs(preset string) []string {
	args := []string{"init", "--preset", preset}
	if m.info.Found && m.info.PolicyPath != "" {
		return append(args, "--force", "--dir", filepath.Dir(m.info.PolicyPath))
	}
	return args
}

func (m landingModel) runPreset() (tea.Model, tea.Cmd) {
	name := presetOptions[m.presetIdx].Name
	if m.info.Found && !m.confirmReplace {
		m.confirmReplace = true
		return m, nil
	}
	return m.startRun(m.presetArgs(name))
}

func (m *landingModel) startRun(args []string) (tea.Model, tea.Cmd) {
	m.runID++
	m.runTitle = "doupass " + strings.Join(args, " ")
	m.screen = screenRunning
	return *m, tea.Batch(runLandingCommand(m.runID, args), spinnerTick())
}

func (m *landingModel) outputWindow() int {
	h := m.height - 12
	if h < 6 {
		h = 6
	}
	return h
}

func (m *landingModel) scrollBy(delta int) {
	m.scroll += delta
	m.clampScroll()
}

func (m *landingModel) clampScroll() {
	max := len(m.output) - m.outputWindow()
	if max < 0 {
		max = 0
	}
	if m.scroll > max {
		m.scroll = max
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func (m landingModel) View() string {
	switch m.screen {
	case screenPreset:
		return m.presetView()
	case screenRunning:
		return m.runningView()
	case screenOutput:
		return m.outputView()
	default:
		return m.menuView()
	}
}

func (m landingModel) menuView() string {
	labelWidth := 0
	for _, c := range m.choices {
		if n := utf8.RuneCountInString(c.Label); n > labelWidth {
			labelWidth = n
		}
	}
	labelWidth++

	plain := make([]string, len(m.choices))
	rowWidth := 0
	for i, c := range m.choices {
		plain[i] = " " + landingIcons[c.Label] + " " + padRight(c.Label, labelWidth) + "  " + c.Description
		if n := utf8.RuneCountInString(plain[i]); n > rowWidth {
			rowWidth = n
		}
	}
	highlight := lipgloss.NewStyle().Background(landingAccent).Foreground(lipgloss.Color("16")).Bold(true).Width(rowWidth - 1)

	title := titleStyle.Render("◆ doupass") + dimStyle.Render(" · policy guard for AI coding agents")
	if v := m.info.Version; v != "" {
		if len(v) > 24 {
			v = truncateMiddle(v, 24)
		}
		version := dimStyle.Render("v" + v)
		if gap := rowWidth - lipgloss.Width(title) - lipgloss.Width(version); gap > 0 {
			title += strings.Repeat(" ", gap) + version
		}
	}

	var lines []string
	lines = append(lines, title, dividerMark)
	lines = append(lines, m.statusRows()...)
	lines = append(lines, dividerMark, "")
	for i, c := range m.choices {
		if i == m.cursor {
			lines = append(lines, " "+highlight.Render(strings.TrimLeft(plain[i], " ")))
		} else {
			lines = append(lines, " "+iconStyle.Render(landingIcons[c.Label])+" "+padRight(c.Label, labelWidth)+"  "+dimStyle.Render(c.Description))
		}
	}
	if !m.choices[m.cursor].Quit {
		lines = append(lines, "", " "+dimStyle.Render("▶ will run: ")+willStyle.Render("doupass "+strings.Join(m.choices[m.cursor].Args, " ")))
	}

	return framed(lines, rowWidth, "  ↑/↓ move  ·  enter run  ·  q quit")
}

func (m landingModel) presetView() string {
	highlight := lipgloss.NewStyle().Background(landingAccent).Foreground(lipgloss.Color("16")).Bold(true)

	lines := []string{
		titleStyle.Render("◆ doupass") + dimStyle.Render(" · choose a ready-made policy"),
		dividerMark,
	}
	switch {
	case m.info.Invalid:
		lines = append(lines, " "+stateGlyph("fail")+" "+dimStyle.Render("current policy: invalid YAML · ")+truncateMiddle(m.presetTargetPath(), 56))
	case !m.info.Found:
		lines = append(lines, " "+stateGlyph("skip")+" "+dimStyle.Render("current policy: none yet — a preset below writes one instantly"))
	default:
		name := m.info.PolicyName
		switch {
		case m.info.PresetName != "":
			name = m.info.PresetName + " preset"
		case name == "":
			name = "custom"
		}
		lines = append(lines, " "+stateGlyph("ok")+" "+dimStyle.Render(fmt.Sprintf("current policy: %s · %s · ", name, plural(m.info.Rules, "rule")))+truncateMiddle(m.presetTargetPath(), 56))
	}
	lines = append(lines, dividerMark)

	rowWidth := len("choose a ready-made policy") + 4
	for i, p := range presetOptions {
		row := fmt.Sprintf(" %d. %-11s %s", i+1, p.Name, p.Description)
		rowWidth = max(rowWidth, utf8.RuneCountInString(row))
		if i == m.presetIdx {
			lines = append(lines, " "+highlight.Render(strings.TrimSpace(row)))
		} else {
			lines = append(lines, " "+padRight(p.Name, 11)+dimStyle.Render("  "+p.Description))
		}
	}

	sel := presetOptions[m.presetIdx]
	count := plural(presetRuleCounts[sel.Name], "rule")
	if m.info.Found && m.confirmReplace {
		lines = append(lines, "", " "+failStyle.Render(fmt.Sprintf("⚠ %s exists — press enter again to replace it with %s (esc cancels)", m.presetTargetPath(), sel.Name)))
	} else {
		lines = append(lines, "", " "+dimStyle.Render("▶ will run: ")+willStyle.Render(displayArgs(m.presetArgs(sel.Name))))
		lines = append(lines, " "+dimStyle.Render(fmt.Sprintf("%s → %s", count, truncateMiddle(m.presetTargetPath(), 56))))
	}
	return framed(lines, rowWidth, "  1-4 pick · ↑/↓ move · enter write policy · esc back")
}

// displayArgs renders a command line for the "will run" hint, collapsing long
// absolute paths so the frame stays narrow on Windows.
func displayArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		if len(a) > 48 && (filepath.IsAbs(a) || strings.HasPrefix(a, "~")) {
			out[i] = truncateMiddle(a, 48)
		}
	}
	return "doupass " + strings.Join(out, " ")
}

func (m landingModel) runningView() string {
	spinner := willStyle.Render(spinnerFrames[m.spinner%len(spinnerFrames)])
	lines := []string{
		titleStyle.Render("◆ doupass") + dimStyle.Render(" · working"),
		dividerMark,
		"",
		" " + spinner + " running " + willStyle.Render(m.runTitle) + dimStyle.Render(" …"),
		"",
		" " + dimStyle.Render("the result appears here; the menu never goes away"),
	}
	return framed(lines, lipgloss.Width("running "+m.runTitle+" …")+8, "")
}

func (m landingModel) outputView() string {
	width := 0
	for _, l := range m.output {
		if n := utf8.RuneCountInString(l); n > width {
			width = n
		}
	}
	if width < len(m.runTitle)+4 {
		width = len(m.runTitle) + 4
	}

	lines := []string{
		titleStyle.Render("◆ " + m.runTitle),
		dividerMark,
	}
	window := m.outputWindow()
	total := len(m.output)
	start := m.scroll
	if start > total {
		start = total
	}
	end := start + window
	if end > total {
		end = total
	}
	lines = append(lines, m.output[start:end]...)
	if total > window {
		pos := dimStyle.Render(fmt.Sprintf("  · %d-%d / %d lines  (↑/↓ scroll)", start+1, end, total))
		lines = append(lines, "", pos)
	}
	if m.runErr != "" {
		lines = append(lines, failStyle.Render("✗ "+m.runErr))
	}
	return framed(lines, width, "  ↑/↓ scroll  ·  enter back to menu")
}

const dividerMark = "\x00divider\x00"

// framed renders the lines inside the landing frame with dim dividers kept
// as wide as the widest content line; an empty footer omits the hint line.
func framed(lines []string, width int, footer string) string {
	inner := width
	for _, l := range lines {
		if l == dividerMark {
			continue
		}
		if n := lipgloss.Width(l); n > inner {
			inner = n
		}
	}
	for i, l := range lines {
		if l == dividerMark {
			lines[i] = dimStyle.Render(strings.Repeat("─", inner))
		}
	}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	out := "\n" + frameStyle.Render(b.String()) + "\n"
	if footer != "" {
		out += footerStyle.Render(footer) + "\n"
	}
	return out
}

func (m landingModel) statusRows() []string {
	line := func(label, value string) string {
		return " " + dimStyle.Render(padRight(label, 12)) + "  " + value
	}
	var policyValue string
	switch {
	case m.info.Invalid:
		policyValue = stateGlyph("fail") + " invalid — check with doupass policy lint"
	case !m.info.Found:
		policyValue = stateGlyph("fail") + " not found — pick Create a policy below"
	default:
		name := m.info.PolicyName
		if name == "" {
			name = filepath.Base(m.info.PolicyPath)
		}
		policyValue = stateGlyph("ok") + " " + name + " · " + plural(m.info.Rules, "rule") + " · " + plural(m.info.LintIssues, "lint issue")
	}
	var auditValue string
	switch {
	case !m.info.AuditExists:
		auditValue = stateGlyph("skip") + " no entries yet"
	case m.info.AuditOK:
		auditValue = stateGlyph("ok") + " " + plural(m.info.AuditEntries, "entry") + " · hash chain OK"
	default:
		auditValue = stateGlyph("fail") + " tampered or corrupt — run doupass log verify"
	}
	integrationState := "skip"
	if m.info.Integrations > 0 {
		integrationState = "ok"
	}
	integrationValue := stateGlyph(integrationState) + " " + plural(m.info.Integrations, "integration") + " wired"
	return []string{
		line("policy", policyValue),
		line("audit", auditValue),
		line("integrations", integrationValue),
	}
}

func runLanding(cmd *cobra.Command) error {
	in := cmd.InOrStdin()
	out := cmd.OutOrStdout()
	if !isInteractiveTerminal(in, out) {
		printStaticLanding(out)
		return nil
	}
	program := tea.NewProgram(
		newLandingModel(),
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	return err
}

func isInteractiveTerminal(in io.Reader, out io.Writer) bool {
	inf, ok := in.(*os.File)
	if !ok {
		return false
	}
	outf, ok := out.(*os.File)
	if !ok {
		return false
	}
	inStat, err := inf.Stat()
	if err != nil || inStat.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	outStat, err := outf.Stat()
	if err != nil || outStat.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return true
}

func printStaticLanding(out io.Writer) {
	fmt.Fprint(out, `doupass — local-first policy engine for AI coding agents

Every tool call an agent makes is checked against one policy file and recorded
in a tamper-evident local audit log. Nothing leaves your machine, and no model
is involved in the decision.

Getting started
  doupass doctor                        show the current state
  doupass setup --dry-run               preview what would be wired
  doupass setup                         wire the tools installed on this machine
  doupass init                          create a starter policy here

Policies
  doupass policy lint <file>            catch duplicates, blanket rules, missing reasons
  doupass policy test <file> --tool X --arg k=v

Enforcement (installed into your agents)
  doupass hook claude                   Claude Code PreToolUse adapter
  doupass proxy --server X -- <cmd>     MCP server behind the policy engine
  doupass decide --input <json>         decision endpoint for any integration

Audit
  doupass log tail                      what did agents do?
  doupass log verify                    is the hash chain intact?

Run 'doupass <command> --help' for details.
In a terminal, run 'doupass' with no arguments for an interactive menu.
`)
}

func landingInfoData() landingInfo {
	info := landingInfo{Version: ResolvedVersion(), AuditPath: filepath.Join(homeDir(), ".doupass", "audit.jsonl")}
	if policyPath, err := findPolicyFile(""); err == nil {
		info.Found = true
		info.PolicyPath = policyPath
		info.PresetName = matchingPreset(policyPath)
		if engine, err := loadEngine(policyPath); err != nil {
			info.Invalid = true
		} else {
			info.PolicyName = engine.Policy.Name
			info.Rules = len(engine.Policy.Rules)
			info.LintIssues = len(policy.Lint(engine.Policy))
			if p := engine.Policy.Audit.Path; p != "" {
				info.AuditPath = expandHome(p)
			}
		}
	}
	if st, err := os.Stat(info.AuditPath); err == nil && st.Mode().IsRegular() {
		info.AuditExists = true
		if res, verifyErr := audit.Verify(info.AuditPath); verifyErr == nil {
			info.AuditOK = true
			info.AuditEntries = res.Entries
		}
	}
	if claude.HasHook(filepath.Join(homeDir(), ".claude", "settings.json")) {
		info.Integrations++
	}
	if opencode.PluginInstalled(filepath.Join(homeDir(), ".config", "opencode", "plugin", "doupass.js")) {
		info.Integrations++
	}
	for _, tgt := range setupTargets() {
		switch tgt.Kind {
		case "mcpjson":
			if fileContains(tgt.Path, `"doupass"`) && fileContains(tgt.Path, `"proxy"`) {
				info.Integrations++
			}
		case "codex":
			if codex.HasWrappers(tgt.Path) {
				info.Integrations++
			}
		}
	}
	return info
}
