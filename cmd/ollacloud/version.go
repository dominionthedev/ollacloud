package ollacloud

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dominionthedev/ollacloud/internal/server"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the ollacloud version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ollacloud v%s\n", server.Version)
		},
	}
}
