package main

import (
	"os"

	"github.com/dominionthedev/ollacloud/cmd/ollacloud"
)

func main() {
	if err := ollacloud.Execute(); err != nil {
		os.Exit(1)
	}
}
