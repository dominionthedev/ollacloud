package cmd

import (
    "log/slog"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "ollacloud",
    Short: "ollacloud - Alternative for Ollama",
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        slog.Error("command failed", "err", err)
        os.Exit(1)
    }
}

func init() {
    rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
}
