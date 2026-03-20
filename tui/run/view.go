package run

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Styles ───────────────────────────────────────────────────────────────────

var (
	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	streamingDotStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("99"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))

	activeBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("99"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
)

// View satisfies tea.Model and renders the complete TUI.
func (s State) View() string {
	if !s.ready {
		return "\n  Connecting to ollacloud…\n"
	}

	var b strings.Builder

	// ── Banner ────────────────────────────────────────────────────────────────
	indicator := ""
	if s.streaming {
		indicator = streamingDotStyle.Render(" ●")
	}
	banner := bannerStyle.Render(fmt.Sprintf("  ollacloud  ›  %s%s  ", s.modelName, indicator))
	b.WriteString(banner + "\n")
	b.WriteString(strings.Repeat("─", s.width) + "\n")

	// ── Viewport ──────────────────────────────────────────────────────────────
	b.WriteString(s.viewport.View())
	b.WriteString("\n")

	// ── Input area ────────────────────────────────────────────────────────────
	border := inputBorderStyle
	if !s.streaming {
		border = activeBorderStyle
	}
	inputView := border.Width(s.width - 2).Render(s.textarea.View())
	b.WriteString(inputView)
	b.WriteString("\n")

	// ── Footer hint ───────────────────────────────────────────────────────────
	hint := "Enter to send  •  /help for commands  •  Ctrl+C to quit"
	if s.multiline {
		hint = `Multiline mode — type """ on its own line to send`
	} else if s.streaming {
		hint = "Streaming…  Ctrl+C to quit"
	}
	b.WriteString(hintStyle.Render("  " + hint))

	return b.String()
}
