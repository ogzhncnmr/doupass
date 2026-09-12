package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/ogzhncnmr/doupass/internal/opencode"
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

const landingAccent = lipgloss.Color("#38BDF8")

var (
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(landingAccent)
	iconStyle   = lipgloss.NewStyle().Foreground(landingAccent)
	chipStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(landingAccent).Padding(0, 1)
	warnChip    = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("#F87171")).Padding(0, 1)
	frameStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(landingAccent).Padding(1, 2)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	willStyle   = lipgloss.NewStyle().Bold(true).Foreground(landingAccent)
)

type landingInfo struct {
	PolicyName   string
	Rules        int
	Found        bool
	Invalid      bool
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
	highlight := lipgloss.NewStyle().Background(landingAccent).Foreground(lipgloss.Color("16")).Bold(true).Width(rowWidth)

	var b strings.Builder
	b.WriteString(titleStyle.Render("doupass"))
	b.WriteString(dimStyle.Render("  policy guard for AI coding agents"))
	b.WriteString("\n")
	b.WriteString(m.chips())
	b.WriteString("\n\n")
	for i, c := range m.choices {
		if i == m.cursor {
			b.WriteString(highlight.Render(strings.TrimLeft(plain[i], " ")))
		} else {
			b.WriteString(" " + iconStyle.Render(landingIcons[c.Label]) + " " + padRight(c.Label, labelWidth) + "  " + dimStyle.Render(c.Description))
		}
		b.WriteString("\n")
	}
	if !m.choices[m.cursor].Quit {
		b.WriteString("\n " + dimStyle.Render("▶ will run: ") + willStyle.Render("doupass "+strings.Join(m.choices[m.cursor].Args, " ")) + "\n")
	}
	return "\n" + frameStyle.Render(b.String()) + "\n" + footerStyle.Render("  ↑/↓ move  ·  enter select  ·  q quit") + "\n"
}

func (m landingModel) chips() string {
	if m.info.Invalid {
		return " " + warnChip.Render(" policy invalid ")
	}
	if !m.info.Found {
		return " " + warnChip.Render(" policy not found ")
	}
	parts := []string{
		chipStyle.Render(fmt.Sprintf(" policy · %s · %d rules ", m.info.PolicyName, m.info.Rules)),
		chipStyle.Render(fmt.Sprintf(" integrations · %d ", m.info.Integrations)),
	}
	return " " + strings.Join(parts, " ")
}

func padRight(s string, n int) string {
	if w := utf8.RuneCountInString(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

func runLanding(cmd *cobra.Command) error {
	in := cmd.InOrStdin()
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
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
	final, err := program.Run()
	if err != nil {
		return err
	}
	m, ok := final.(landingModel)
	if !ok || len(m.chosen) == 0 {
		return nil
	}
	return Execute(m.chosen, in, out, errOut)
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
	info := landingInfo{}
	if path, err := findPolicyFile(""); err == nil {
		info.Found = true
		if engine, err := loadEngine(path); err == nil {
			info.PolicyName = engine.Policy.Name
			info.Rules = len(engine.Policy.Rules)
		} else {
			info.Invalid = true
		}
	}
	if claude.HasHook(filepath.Join(homeDir(), ".claude", "settings.json")) {
		info.Integrations++
	}
	if opencode.PluginInstalled(filepath.Join(homeDir(), ".config", "opencode", "plugin", "doupass.js")) {
		info.Integrations++
	}
	for _, tgt := range setupTargets() {
		if tgt.Kind == "mcpjson" && fileContains(tgt.Path, `"doupass"`) && fileContains(tgt.Path, `"proxy"`) {
			info.Integrations++
		}
	}
	return info
}
