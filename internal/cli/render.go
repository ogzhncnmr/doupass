package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

const landingAccent = lipgloss.Color("#38BDF8")

var (
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(landingAccent)
	iconStyle   = lipgloss.NewStyle().Foreground(landingAccent)
	frameStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(landingAccent).Padding(1, 2)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	willStyle   = lipgloss.NewStyle().Bold(true).Foreground(landingAccent)
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	failStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	planStyle   = lipgloss.NewStyle().Foreground(landingAccent)
)

func stateGlyph(state string) string {
	switch state {
	case "ok", "done":
		return okStyle.Render("✓")
	case "fail":
		return failStyle.Render("✗")
	case "plan":
		return planStyle.Render("+")
	default:
		return dimStyle.Render("•")
	}
}

func padRight(s string, n int) string {
	if w := utf8.RuneCountInString(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

// truncateMiddle keeps the head and tail of long values (paths stay readable:
// drive, home, and file name survive) and hides the middle behind an ellipsis.
func truncateMiddle(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	const dots = "…"
	tail := max / 3
	head := max - tail - utf8.RuneCountInString(dots)
	if head < 1 {
		return string(runes[len(runes)-max:])
	}
	return string(runes[:head]) + dots + string(runes[len(runes)-tail:])
}

// plural handles the regular English plural and the y→ies rule; every noun
// used with it (rule, rule set, entry, tool, integration, lint issue) is
// covered by those two forms.
func plural(n int, singular string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	if strings.HasSuffix(singular, "y") && len(singular) > 1 && !isVowel(singular[len(singular)-2]) {
		return fmt.Sprintf("%d %sies", n, singular[:len(singular)-1])
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

func isVowel(c byte) bool {
	switch c {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}
