package ollacloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/api"
)

func psCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List models with active cloud requests",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(daemonEndpoint("/api/ps")) //nolint:noctx
			if err != nil {
				return fmt.Errorf("cannot reach ollacloud daemon — is it running? (`ollacloud serve`)\n  %w", err)
			}
			defer resp.Body.Close()

			var ps api.PSResponse
			if err := json.NewDecoder(resp.Body).Decode(&ps); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}
			renderPSTable(ps.Models)
			return nil
		},
	}
}

func renderPSTable(models []api.RunningModel) {
	if len(models) == 0 {
		fmt.Fprintln(os.Stdout, "No active cloud requests.")
		return
	}
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	headers := []string{"NAME", "ID", "EXPIRES"}
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
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			m.Model,
			dimStyle.Render(id),
			dimStyle.Render(m.ExpiresAt),
		)
	}
	w.Flush()
}
