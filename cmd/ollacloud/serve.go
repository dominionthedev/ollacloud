package ollacloud

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/auth"
	"github.com/dominionthedev/ollacloud/internal/config"
	"github.com/dominionthedev/ollacloud/internal/env"
	"github.com/dominionthedev/ollacloud/internal/server"
)

func serveCmd() *cobra.Command {
	var flagKey string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the ollacloud daemon",
		Long: `Start the Ollama-compatible proxy daemon.

ollacloud binds to OLLACLOUD_HOST (default 127.0.0.1:11434) and forwards all
Ollama and OpenAI-compatible API requests to Ollama Cloud, injecting your API
key automatically. Any app already built for Ollama works with zero changes.

Environment variables:

  OLLACLOUD_API_KEY     API key for Ollama Cloud (also: OLLAMA_API_KEY)
  OLLACLOUD_HOST        Daemon bind address       (also: OLLAMA_HOST)
  OLLACLOUD_UPSTREAM    Cloud base URL            (default: https://ollama.com)
  OLLACLOUD_ORIGINS     Extra allowed CORS origins (also: OLLAMA_ORIGINS)
  OLLACLOUD_KEEP_ALIVE  Active-model expiry window (also: OLLAMA_KEEP_ALIVE)
  OLLACLOUD_MAX_QUEUE   Max queued requests        (also: OLLAMA_MAX_QUEUE)
  OLLACLOUD_DEBUG       Enable debug logging       (also: OLLAMA_DEBUG)

Run 'ollacloud serve --env' to see current resolved values.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// --env: print resolved env table and exit (like ollama serve --help shows vars)
			if envFlag, _ := cmd.Flags().GetBool("env"); envFlag {
				printEnvTable()
				return nil
			}

			// Resolve API key: flag > env vars (via auth.Resolve) > config file > prompt
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			key, err := auth.Resolve(auth.ResolveOptions{
				FlagValue:   flagKey,
				AllowPrompt: true,
			}, cfg)
			if err != nil {
				return err
			}

			return server.Run(server.Config{
				APIKey:      key,
				UpstreamURL: env.UpstreamURL(),
			})
		},
	}

	cmd.Flags().StringVarP(&flagKey, "key", "k", "", "Ollama Cloud API key (overrides env and config)")
	cmd.Flags().Bool("env", false, "Print resolved environment variable values and exit")
	return cmd
}

// printEnvTable renders the resolved env var table to stdout.
func printEnvTable() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VARIABLE\tVALUE\tDESCRIPTION")
	fmt.Fprintln(w, "--------\t-----\t-----------")
	for _, v := range env.Table() {
		fmt.Fprintf(w, "%s\t%s\t%s\n", v.Name, v.Value, v.Description)
	}
	w.Flush()
}
