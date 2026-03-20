package ollacloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/api"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List models in your Ollama Cloud account",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(daemonEndpoint("/api/tags")) //nolint:noctx
			if err != nil {
				return fmt.Errorf("cannot reach ollacloud daemon — is it running? (`ollacloud serve`)\n  %w", err)
			}
			defer resp.Body.Close()

			var tags api.TagsResponse
			if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}
			renderModelTable(tags.Models)
			return nil
		},
	}
}

func renderModelTable(models []api.ModelInfo) {
	if len(models) == 0 {
		fmt.Fprintln(os.Stdout, "No models found in your cloud account.")
		fmt.Fprintln(os.Stdout, "Pull one with: ollacloud pull <model>")
		return
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	headers := []string{"NAME", "ID", "SIZE", "MODIFIED"}
	styled := make([]string, len(headers))
	for i, h := range headers {
		styled[i] = headerStyle.Render(h)
	}
	fmt.Fprintln(w, strings.Join(styled, "\t"))
	for _, m := range models {
		id := m.Digest
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			m.Name,
			dimStyle.Render(id),
			formatBytes(m.Size),
			dimStyle.Render(formatModified(m.ModifiedAt)),
		)
	}
	w.Flush()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatModified(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}
