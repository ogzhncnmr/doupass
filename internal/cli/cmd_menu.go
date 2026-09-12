package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	{Label: "Create a policy", Description: "Write a starter doupass.yml in this directory", Args: []string{"init"}},
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

type landingInfo struct {
	Version      string
	PolicyPath   string
	PolicyName   string
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
	chosen  []string
	quit    bool
}

func newLandingModel() landingModel {
	return landingModel{choices: landingChoices, info: landingInfoData()}
}

func (m landingModel) Init() tea.Cmd { return nil }

func (m landingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = (m.cursor + len(m.choices) - 1) % len(m.choices)
		case "down", "j":
			m.cursor = (m.cursor + 1) % len(m.choices)
		case "enter":
			choice := m.choices[m.cursor]
			if choice.Quit {
				m.quit = true
				return m, tea.Quit
			}
			m.chosen = choice.Args
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.quit = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m landingModel) View() string {
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

	inner := rowWidth
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
	return "\n" + frameStyle.Render(b.String()) + "\n" + footerStyle.Render("  ↑/↓ move  ·  enter run  ·  q quit") + "\n"
}

const dividerMark = "\x00divider\x00"

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
	errOut := cmd.ErrOrStderr()
	if !isInteractiveTerminal(in, out) {
		printStaticLanding(out)
		return nil
	}
	for {
		choice, quit, err := selectLanding(in, out)
		if err != nil || quit {
			return err
		}
		args := choice
		if choice[0] == "init" {
			preset, ok := choosePreset(in, out)
			if !ok {
				continue
			}
			args = []string{"init", "--preset", preset}
		}
		fmt.Fprintf(out, "\n%s %s\n\n", iconStyle.Render("◆"), willStyle.Render("doupass "+strings.Join(args, " ")))
		if err := Execute(args, in, out, errOut); err != nil {
			fmt.Fprintf(errOut, "%s\n", failStyle.Render("✗ "+err.Error()))
		}
		pauseForReturn(in, out)
	}
}

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

func choosePreset(in io.Reader, out io.Writer) (string, bool) {
	fmt.Fprintln(out, "Choose a ready-made policy:")
	for i, p := range presetOptions {
		fmt.Fprintf(out, "  %d. %-11s %s\n", i+1, p.Name, dimStyle.Render(p.Description))
	}
	fmt.Fprint(out, "\npreset [1-4, enter = starter, q = cancel]: ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "", "1", "starter":
		return "starter", true
	case "2", "minimal":
		return "minimal", true
	case "3", "locked-down", "lockeddown":
		return "locked-down", true
	case "4", "red-team", "redteam":
		return "red-team", true
	default:
		fmt.Fprintf(out, "%s unknown preset — back to the menu\n", failStyle.Render("✗"))
		return "", false
	}
}

func selectLanding(in io.Reader, out io.Writer) ([]string, bool, error) {
	program := tea.NewProgram(
		newLandingModel(),
		tea.WithInput(in),
		tea.WithOutput(out),
		tea.WithAltScreen(),
	)
	final, err := program.Run()
	if err != nil {
		return nil, false, err
	}
	m, ok := final.(landingModel)
	if !ok || m.quit || len(m.chosen) == 0 {
		return nil, true, nil
	}
	return m.chosen, false, nil
}

func pauseForReturn(in io.Reader, out io.Writer) {
	fmt.Fprint(out, dimStyle.Render("\npress Enter to return to the menu…"))
	r := bufio.NewReader(in)
	_, _ = r.ReadString('\n')
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
