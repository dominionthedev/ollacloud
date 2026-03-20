// Package ollacloud is the root of the ollacloud CLI.
// It wires all subcommands under a single cobra root command.
package ollacloud

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ollacloud",
	Short: "Ollama-compatible cloud proxy — use Ollama Cloud without downloading Ollama",
	Long: `ollacloud is a drop-in replacement for the Ollama server and CLI.
It proxies all Ollama API calls to Ollama Cloud, so any app already
built for Ollama works with zero changes.

Set your API key once:
  export OLLAMA_API_KEY=<your key>
  ollacloud serve

Then use it exactly like Ollama:
  ollacloud run gemma3
  ollacloud pull deepseek-v3:cloud
  ollacloud list
`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Execute runs the root command. Called from main.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(
		serveCmd(),
		runCmd(),
		pullCmd(),
		pushCmd(),
		listCmd(),
		psCmd(),
		showCmd(),
		rmCmd(),
		cpCmd(),
		createCmd(),
		stopCmd(),
		authCmd(),
		versionCmd(),
	)
}
