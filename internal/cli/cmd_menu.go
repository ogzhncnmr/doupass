package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

type landingModel struct {
	choices []landingChoice
	cursor  int
	status  string
	chosen  []string
	quit    bool
}

func newLandingModel(status string) landingModel {
	return landingModel{choices: landingChoices, status: status}
}

func (m landingModel) Init() tea.Cmd { return nil }

func (m landingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
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
	return m, nil
}

func (m landingModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	pointer := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	var b strings.Builder
	b.WriteString("\n  " + title.Render("doupass") + dim.Render("  —  policy guard for AI coding agents") + "\n")
	b.WriteString("  " + dim.Render(m.status) + "\n\n")
	for i, c := range m.choices {
		prefix := "    "
		label := fmt.Sprintf("%-24s", c.Label)
		desc := dim.Render(c.Description)
		if i == m.cursor {
			prefix = pointer.Render("  ▸ ")
			label = selected.Render(label)
			desc = c.Description
		}
		b.WriteString("  " + prefix + label + " " + desc + "\n")
	}
	b.WriteString("\n  " + dim.Render("↑/↓ move  •  enter select  •  q quit") + "\n")
	return b.String()
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
		newLandingModel(landingStatus()),
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

func landingStatus() string {
	var parts []string
	if path, err := findPolicyFile(""); err == nil {
		if engine, err := loadEngine(path); err == nil {
			parts = append(parts, fmt.Sprintf("policy %s (%d rules)", engine.Policy.Name, len(engine.Policy.Rules)))
		} else {
			parts = append(parts, "policy invalid")
		}
	} else {
		parts = append(parts, "policy not found")
	}
	count := 0
	if claude.HasHook(filepath.Join(homeDir(), ".claude", "settings.json")) {
		count++
	}
	if opencode.PluginInstalled(filepath.Join(homeDir(), ".config", "opencode", "plugin", "doupass.js")) {
		count++
	}
	for _, tgt := range setupTargets() {
		if tgt.Kind == "mcpjson" && fileContains(tgt.Path, `"doupass"`) && fileContains(tgt.Path, `"proxy"`) {
			count++
		}
	}
	parts = append(parts, fmt.Sprintf("%d integrations", count))
	return strings.Join(parts, "  ·  ")
}
