package ollacloud

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/config"
)

func stopServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop-server",
		Short: "Stop the background ollacloud daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := config.DataDir()
			if err != nil {
				return err
			}

			pidFile := filepath.Join(dataDir, "ollacloud.pid")
			data, err := os.ReadFile(pidFile)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no background server running (PID file not found)")
				}
				return fmt.Errorf("reading PID file: %w", err)
			}

			pid, err := strconv.Atoi(string(data))
			if err != nil {
				return fmt.Errorf("invalid PID in %s", pidFile)
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("could not find process %d: %w", pid, err)
			}

			fmt.Printf("Sending SIGTERM to process %d...\n", pid)
			if err := process.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("sending SIGTERM: %w", err)
			}

			// Clean up PID file
			_ = os.Remove(pidFile)
			fmt.Println("✓ Server stopped")
			return nil
		},
	}
}
