package ollacloud

import (
	"fmt"

	"github.com/dominionthedev/ollacloud/internal/env"
)

// daemonURL returns the base URL of the local ollacloud daemon,
// derived from OLLACLOUD_HOST / OLLAMA_HOST / default.
// All CLI commands that talk to the running daemon use this.
func daemonURL() string {
	return "http://" + env.Host()
}

// daemonEndpoint returns the full URL for a daemon API path.
func daemonEndpoint(path string) string {
	return fmt.Sprintf("%s%s", daemonURL(), path)
}
