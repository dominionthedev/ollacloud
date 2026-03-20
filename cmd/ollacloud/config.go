package ollacloud

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/config"
	"github.com/dominionthedev/ollacloud/internal/env"
)

// configCmd is the root `ollacloud config` command.
// Subcommands: list, get, set, unset, path
func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and manage ollacloud configuration",
		Long: `Manage the ollacloud config file at ~/.config/ollacloud/config.toml.

The config file holds persistent overrides. Environment variables always
take priority over config file values at runtime.

Configurable keys:
  api_key        Ollama Cloud API key  (env: OLLACLOUD_API_KEY, OLLAMA_API_KEY)
  upstream_url   Ollama Cloud base URL (env: OLLACLOUD_UPSTREAM)
  host           Daemon bind address   (env: OLLACLOUD_HOST, OLLAMA_HOST)`,
	}
	cmd.AddCommand(
		configListCmd(),
		configGetCmd(),
		configSetCmd(),
		configUnsetCmd(),
		configPathCmd(),
	)
	return cmd
}

// ollacloud config list  — show all config values with their sources
func configListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all config values and their sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := config.LoadFile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			r, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			keyStyle := lipgloss.NewStyle().Bold(true)
			envStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
			fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("71"))
			defaultStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
			dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				keyStyle.Render("KEY"),
				keyStyle.Render("VALUE"),
				keyStyle.Render("SOURCE"),
			)
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				strings.Repeat("─", 15),
				strings.Repeat("─", 30),
				strings.Repeat("─", 10),
			)

			printRow := func(key, envVar, fileVal, resolvedVal, defaultVal string) {
				var src, display string
				switch {
				case env.APIKey() != "" && key == "api_key":
					display = maskKey(resolvedVal)
					src = envStyle.Render("env")
				case resolvedVal != defaultVal && resolvedVal == fileVal:
					if key == "api_key" {
						display = maskKey(resolvedVal)
					} else {
						display = resolvedVal
					}
					src = fileStyle.Render("file")
				case resolvedVal != "" && envVar != "" && os.Getenv(envVar) != "":
					display = resolvedVal
					src = envStyle.Render("env")
				default:
					if key == "api_key" && resolvedVal != "" {
						display = maskKey(resolvedVal)
					} else {
						display = resolvedVal
					}
					if resolvedVal == defaultVal {
						src = defaultStyle.Render("default")
					} else {
						src = fileStyle.Render("file")
					}
				}
				if display == "" {
					display = dimStyle.Render("(not set)")
					src = dimStyle.Render("—")
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", key, display, src)
			}

			printRow("api_key",      "OLLACLOUD_API_KEY", f.APIKey,      r.APIKey,      "")
			printRow("host",         "OLLACLOUD_HOST",    f.Host,        r.Host,        "127.0.0.1:11434")
			printRow("upstream_url", "OLLACLOUD_UPSTREAM", f.UpstreamURL, r.UpstreamURL, "https://ollama.com")

			w.Flush()

			fmt.Fprintln(os.Stdout)
			dimStyle2 := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
			path, _ := config.Path()
			fmt.Fprintln(os.Stdout, dimStyle2.Render("Config file: "+path))
			fmt.Fprintln(os.Stdout, dimStyle2.Render("Env vars override file values at runtime."))
			return nil
		},
	}
}

// ollacloud config get <key>  — print a single value
func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a single config value (resolved with env var overlay)",
		Args:  cobra.ExactArgs(1),
		ValidArgs: []string{"api_key", "host", "upstream_url"},
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			key := strings.ToLower(strings.TrimSpace(args[0]))
			switch key {
			case "api_key":
				if r.APIKey == "" {
					fmt.Fprintln(os.Stdout, "(not set)")
				} else {
					fmt.Fprintln(os.Stdout, maskKey(r.APIKey))
				}
			case "host":
				fmt.Fprintln(os.Stdout, r.Host)
			case "upstream_url":
				fmt.Fprintln(os.Stdout, r.UpstreamURL)
			default:
				return fmt.Errorf("unknown key %q — valid keys: api_key, host, upstream_url", key)
			}
			return nil
		},
	}
}

// ollacloud config set <key> <value>  — write a value to the config file
func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "set <key> <value>",
		Short:     "Set a config value in the config file",
		Args:      cobra.ExactArgs(2),
		ValidArgs: []string{"api_key", "host", "upstream_url"},
		Example: `  ollacloud config set host 0.0.0.0:11434
  ollacloud config set upstream_url https://ollama.com
  ollacloud config set api_key sk-your-key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToLower(strings.TrimSpace(args[0]))
			val := strings.TrimSpace(args[1])

			if val == "" {
				return fmt.Errorf("value must not be empty — use `ollacloud config unset %s` to clear it", key)
			}

			f, err := config.LoadFile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			switch key {
			case "api_key":
				f.APIKey = val
			case "host":
				f.Host = val
			case "upstream_url":
				// Basic sanity: must look like a URL
				if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
					return fmt.Errorf("upstream_url must start with http:// or https://")
				}
				f.UpstreamURL = strings.TrimRight(val, "/")
			default:
				return fmt.Errorf("unknown key %q — valid keys: api_key, host, upstream_url", key)
			}

			if err := config.SaveFile(f); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			path, _ := config.Path()
			display := val
			if key == "api_key" {
				display = maskKey(val)
			}
			fmt.Fprintf(os.Stdout, "✓ %s = %s  (saved to %s)\n", key, display, path)
			return nil
		},
	}
}

// ollacloud config unset <key>  — clear a value from the config file
func configUnsetCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "unset <key>",
		Short:     "Clear a config value from the config file (revert to default or env var)",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"api_key", "host", "upstream_url"},
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToLower(strings.TrimSpace(args[0]))

			f, err := config.LoadFile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			switch key {
			case "api_key":
				f.APIKey = ""
			case "host":
				f.Host = ""
			case "upstream_url":
				f.UpstreamURL = ""
			default:
				return fmt.Errorf("unknown key %q — valid keys: api_key, host, upstream_url", key)
			}

			if err := config.SaveFile(f); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}

			fmt.Fprintf(os.Stdout, "✓ %s cleared (will use env var or default)\n", key)
			return nil
		},
	}
}

// ollacloud config path  — print the config file path
func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the path to the config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.Path()
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, path)
			return nil
		},
	}
}

// maskKey shows only the last 4 characters of an API key.
func maskKey(key string) string {
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}
