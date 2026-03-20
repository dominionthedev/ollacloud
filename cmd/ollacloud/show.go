package ollacloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/api"
)

func showCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "show <model>",
		Short: "Show information about a model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, _ := json.Marshal(api.ShowRequest{Model: args[0], Verbose: verbose})
			resp, err := http.Post(daemonEndpoint("/api/show"), "application/json", bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("cannot reach daemon: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				var errResp api.ErrorResponse
				json.NewDecoder(resp.Body).Decode(&errResp) //nolint:errcheck
				return fmt.Errorf("show failed: %s", errResp.Error)
			}

			var show api.ShowResponse
			if err := json.NewDecoder(resp.Body).Decode(&show); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}
			renderShowCard(args[0], show)
			return nil
		},
	}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show extended model information")
	return cmd
}

func renderShowCard(name string, s api.ShowResponse) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1)
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("243")).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1).MarginTop(1)

	fmt.Fprintln(os.Stdout, titleStyle.Render("  "+name))

	row := func(label, value string) string {
		if value == "" {
			return ""
		}
		return labelStyle.Render(label) + valueStyle.Render(value) + "\n"
	}

	content := ""
	content += row("Architecture", s.Details.Family)
	content += row("Parameters", s.Details.ParameterSize)
	content += row("Quantization", s.Details.QuantizationLevel)
	content += row("Format", s.Details.Format)
	content += row("Modified", s.ModifiedAt)
	if len(s.Capabilities) > 0 {
		caps := ""
		for i, c := range s.Capabilities {
			if i > 0 {
				caps += ", "
			}
			caps += c
		}
		content += row("Capabilities", caps)
	}
	if s.Parameters != "" {
		content += row("Parameters", s.Parameters)
	}
	fmt.Fprintln(os.Stdout, boxStyle.Render(content))
}
