package ollacloud

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/env"
	"github.com/dominionthedev/ollacloud/tui/run"
)

func runCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <model>",
		Short: "Start an interactive chat session with a cloud model",
		Long: `Opens a full interactive TUI chat session with the specified cloud model.

The ollacloud daemon must be running first:
  ollacloud serve

Examples:
  ollacloud run gemma3
  ollacloud run gpt-oss:120b-cloud
  ollacloud run deepseek-v3.1:671b-cloud`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			state := run.New(args[0], env.Host())

			p := tea.NewProgram(
				state,
				tea.WithAltScreen(),
				tea.WithMouseCellMotion(),
			)

			if _, err := p.Run(); err != nil {
				fmt.Fprintln(os.Stderr, "TUI error:", err)
				return err
			}
			return nil
		},
	}
}
