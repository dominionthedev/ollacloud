package ollacloud

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/auth"
	"github.com/dominionthedev/ollacloud/internal/config"
	"github.com/dominionthedev/ollacloud/internal/env"
	"github.com/dominionthedev/ollacloud/internal/server"
)

func serveCmd() *cobra.Command {
	var flagKey string
	var background bool

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
				AllowPrompt: !background, // No prompt in background mode
			}, cfg)
			if err != nil {
				return err
			}

			if background {
				return startBackground(key)
			}

			return server.Run(server.Config{
				APIKey:      key,
				UpstreamURL: env.UpstreamURL(),
			})
		},
	}

	cmd.Flags().StringVarP(&flagKey, "key", "k", "", "Ollama Cloud API key (overrides env and config)")
	cmd.Flags().Bool("env", false, "Print resolved environment variable values and exit")
	cmd.Flags().BoolVar(&background, "background", false, "Run the server in the background")
	return cmd
}

func startBackground(key string) error {
	dataDir, err := config.DataDir()
	if err != nil {
		return err
	}

	pidFile := filepath.Join(dataDir, "ollacloud.pid")
	logFile := filepath.Join(dataDir, "server.log")

	// Check if already running
	if data, err := os.ReadFile(pidFile); err == nil {
		pid, _ := strconv.Atoi(string(data))
		if processExists(pid) {
			return fmt.Errorf("server already running (PID %d). Use 'ollacloud stop-server' to stop it.", pid)
		}
	}

	// Prepare background command
	// We re-run ourselves without the --background flag
	args := []string{"serve"}
	if key != "" {
		args = append(args, "--key", key)
	}

	cmd := exec.Command(os.Args[0], args...)
	// Set environment variables to match current ones
	cmd.Env = os.Environ()

	// Redirect stdout and stderr to log file
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	cmd.Stdout = logF
	cmd.Stderr = logF

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting background process: %w", err)
	}

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}

	fmt.Printf("✓ ollacloud server started in background (PID %d)\n", cmd.Process.Pid)
	fmt.Printf("✓ Logs: %s\n", logFile)
	return nil
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Need to send signal 0 to check if it's alive.
	err = process.Signal(syscall.Signal(0))
	return err == nil
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
