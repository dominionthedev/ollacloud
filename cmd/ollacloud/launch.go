package ollacloud

import (
	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/launch"
)

func launchCmd() *cobra.Command {
	var configOnly bool

	cmd := &cobra.Command{
		Use:   "launch <tool>",
		Short: "Launch an IDE integration or coding tool",
		Long: `Configure and start an integration pointing at ollacloud.

Supported tools:
  claude      Claude Code
  opencode    OpenCode
  codex       Codex
  cline       Cline
  droid       Droid

Example:
  ollacloud launch claude
  ollacloud launch claude -- --model gemma3
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tool := args[0]
			var toolArgs []string

			// If -- was used, everything after it is passed to the tool
			dashIndex := cmd.ArgsLenAtDash()
			if dashIndex != -1 {
				toolArgs = args[dashIndex:]
			} else if len(args) > 1 {
				toolArgs = args[1:]
			}

			return launch.Run(tool, toolArgs, configOnly)
		},
	}

	cmd.Flags().BoolVar(&configOnly, "config", false, "Print configuration instead of launching")
	return cmd
}
