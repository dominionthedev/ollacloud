package ollacloud

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/auth"
	"github.com/dominionthedev/ollacloud/internal/config"
	"github.com/dominionthedev/ollacloud/internal/env"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Ollama Cloud authentication",
	}
	cmd.AddCommand(authSetCmd(), authRemoveCmd(), authStatusCmd())
	return cmd
}

func authSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set",
		Short: "Save an API key to the config file",
		Long: `Reads your Ollama Cloud API key and saves it to:
  ~/.config/ollacloud/config.toml

Get your key at: https://ollama.com/settings/keys`,
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := auth.Resolve(auth.ResolveOptions{AllowPrompt: true}, config.Resolved{})
			if err != nil {
				return err
			}
			if err := auth.Validate(key); err != nil {
				return err
			}
			f, err := config.LoadFile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			f.APIKey = key
			if err := config.SaveFile(f); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			path, _ := config.Path()
			fmt.Fprintf(os.Stdout, "✓ API key saved to %s\n", path)
			return nil
		},
	}
}

func authRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Remove the stored API key from the config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := config.LoadFile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			f.APIKey = ""
			if err := config.SaveFile(f); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Fprintln(os.Stdout, "✓ API key removed from config")
			return nil
		},
	}
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show where the current API key is coming from",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, _ := config.Path()
			f, _ := config.LoadFile()

			switch {
			case env.APIKey() != "":
				src := "OLLACLOUD_API_KEY"
				if os.Getenv("OLLACLOUD_API_KEY") == "" {
					src = "OLLAMA_API_KEY"
				}
				fmt.Fprintf(os.Stdout, "✓ Using key from %s\n", src)
			case f.APIKey != "":
				fmt.Fprintf(os.Stdout, "✓ Using key stored in %s\n", path)
			default:
				fmt.Fprintln(os.Stdout, "✗ No API key found — run `ollacloud auth set` or set OLLACLOUD_API_KEY")
				os.Exit(1)
			}
			return nil
		},
	}
}
