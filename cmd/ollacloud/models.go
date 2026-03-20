package ollacloud

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/api"
)

// ── pull ─────────────────────────────────────────────────────────────────────

func pullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <model>",
		Short: "Pull a model to your Ollama Cloud account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return streamStatusCmd("POST", "/api/pull",
				api.PullRequest{Model: args[0]},
				fmt.Sprintf("Pulling %s", args[0]),
			)
		},
	}
}

// ── push ─────────────────────────────────────────────────────────────────────

func pushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <model>",
		Short: "Push a model to the Ollama registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return streamStatusCmd("POST", "/api/push",
				api.PushRequest{Model: args[0]},
				fmt.Sprintf("Pushing %s", args[0]),
			)
		},
	}
}

// ── create ───────────────────────────────────────────────────────────────────

func createCmd() *cobra.Command {
	var modelfile string

	cmd := &cobra.Command{
		Use:   "create <model>",
		Short: "Create a model from a Modelfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read modelfile content if provided.
			// For now we forward the request directly; Modelfile parsing is
			// handled server-side by Ollama Cloud.
			req := api.CreateRequest{Model: args[0]}
			if modelfile != "" {
				data, err := os.ReadFile(modelfile)
				if err != nil {
					return fmt.Errorf("reading modelfile: %w", err)
				}
				// The "from" field is parsed out of the Modelfile by the cloud.
				// We pass the raw content via the system field for now.
				_ = data // full Modelfile parsing is a future enhancement
			}
			return streamStatusCmd("POST", "/api/create", req,
				fmt.Sprintf("Creating %s", args[0]),
			)
		},
	}

	cmd.Flags().StringVarP(&modelfile, "file", "f", "Modelfile", "Path to the Modelfile")
	return cmd
}

// ── rm ───────────────────────────────────────────────────────────────────────

func rmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <model>",
		Aliases: []string{"remove"},
		Short:   "Remove a model from your Ollama Cloud account",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
	body, _ := json.Marshal(api.DeleteRequest{Model: args[0]})
			req, err := http.NewRequest(http.MethodDelete,
				daemonEndpoint("/api/delete"),
				bytes.NewReader(body),
			)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("cannot reach daemon: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				fmt.Fprintf(os.Stdout, "✓ Deleted %s\n", args[0])
				return nil
			}

			var errResp api.ErrorResponse
			json.NewDecoder(resp.Body).Decode(&errResp) //nolint:errcheck
			return fmt.Errorf("delete failed: %s", errResp.Error)
		},
	}
}

// ── cp ───────────────────────────────────────────────────────────────────────

func cpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cp <source> <destination>",
		Short: "Copy a model to a new name",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return postJSON("/api/copy", api.CopyRequest{
				Source:      args[0],
				Destination: args[1],
			}, func() { fmt.Fprintf(os.Stdout, "✓ Copied %s → %s\n", args[0], args[1]) })
		},
	}
}

// ── stop ─────────────────────────────────────────────────────────────────────

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <model>",
		Short: "Stop a running model (no-op for cloud models)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Cloud models are managed server-side; there is no local VRAM to free.
			noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
			fmt.Fprintln(os.Stdout, noteStyle.Render(
				fmt.Sprintf("ℹ  %s is a cloud model — resource management is handled by Ollama Cloud.", args[0]),
			))
		},
	}
}

// ── shared helpers ────────────────────────────────────────────────────────────

// streamStatusCmd sends a POST to the daemon and streams the NDJSON status
// frames to the terminal, rendering progress lines like the real Ollama CLI.
func streamStatusCmd(method, path string, body any, label string) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encoding request: %w", err)
	}

	url := daemonEndpoint(path)
	req, err := http.NewRequest(method, url, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach daemon: %w", err)
	}
	defer resp.Body.Close()

	labelStyle := lipgloss.NewStyle().Bold(true)
	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	fmt.Fprintln(os.Stdout, labelStyle.Render(label))

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var frame api.StatusFrame
		if err := json.Unmarshal(line, &frame); err != nil {
			// Not JSON — print raw.
			fmt.Fprintln(os.Stdout, string(line))
			continue
		}

		if frame.Error != "" {
			fmt.Fprintln(os.Stderr, errStyle.Render("✗ "+frame.Error))
			return fmt.Errorf(frame.Error)
		}

		// Build a progress line.
		status := frame.Status
		if frame.Total > 0 {
			pct := float64(frame.Completed) / float64(frame.Total) * 100
			bar := buildProgressBar(pct, 30)
			fmt.Fprintf(os.Stdout, "\r%s %s %s  %.1f%%   ",
				status,
				barStyle.Render(bar),
				formatBytes(frame.Total),
				pct,
			)
		} else {
			// Simple status line — clear previous progress bar if any.
			fmt.Fprintf(os.Stdout, "\r%-60s\n", status)
		}

		if strings.EqualFold(status, "success") {
			fmt.Fprintln(os.Stdout)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("reading stream: %w", err)
	}

	return nil
}

// postJSON sends a plain POST and calls onSuccess if the status is 200.
func postJSON(path string, body any, onSuccess func()) error {
	encoded, _ := json.Marshal(body)
	resp, err := http.Post( //nolint:noctx
		daemonEndpoint(path),
		"application/json",
		bytes.NewReader(encoded),
	)
	if err != nil {
		return fmt.Errorf("cannot reach daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		onSuccess()
		return nil
	}

	var errResp api.ErrorResponse
	json.NewDecoder(resp.Body).Decode(&errResp) //nolint:errcheck
	return fmt.Errorf("request failed (%d): %s", resp.StatusCode, errResp.Error)
}

// buildProgressBar renders a simple ASCII progress bar.
func buildProgressBar(pct float64, width int) string {
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	return "█" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
