package launch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dominionthedev/ollacloud/internal/env"
)

type Tool struct {
	Name        string
	PackageName string
	Command     string
	Env         func(host string) []string
}

var Tools = map[string]Tool{
	"claude": {
		Name:        "Claude Code",
		PackageName: "@anthropic-ai/claude-code",
		Command:     "claude",
		Env: func(host string) []string {
			return []string{
				fmt.Sprintf("ANTHROPIC_BASE_URL=http://%s/api/anthropic", host),
			}
		},
	},
	"opencode": {
		Name:        "OpenCode",
		PackageName: "opencode", // hypothetical
		Command:     "opencode",
		Env: func(host string) []string {
			return []string{
				fmt.Sprintf("OLLAMA_HOST=%s", host),
			}
		},
	},
	"codex": {
		Name:    "Codex",
		Command: "codex",
		Env: func(host string) []string {
			return []string{
				fmt.Sprintf("OLLAMA_HOST=%s", host),
			}
		},
	},
	"cline": {
		Name:    "Cline",
		Command: "cline",
		Env: func(host string) []string {
			return []string{
				fmt.Sprintf("OLLAMA_HOST=%s", host),
			}
		},
	},
	"droid": {
		Name:    "Droid",
		Command: "droid",
		Env: func(host string) []string {
			return []string{
				fmt.Sprintf("OLLAMA_HOST=%s", host),
			}
		},
	},
}

func Run(toolName string, args []string, configOnly bool) error {
	tool, ok := Tools[strings.ToLower(toolName)]
	if !ok {
		return fmt.Errorf("unknown tool: %s. Supported tools: claude, opencode, codex, cline, droid", toolName)
	}

	host := env.Host()

	toolEnv := tool.Env(host)

	if configOnly {
		fmt.Printf("✓ To configure %s, set the following environment variables:\n", tool.Name)
		for _, e := range toolEnv {
			fmt.Printf("  export %s\n", e)
		}
		return nil
	}

	// Check if command exists
	_, err := exec.LookPath(tool.Command)
	if err != nil {
		return fmt.Errorf("%s not found in PATH. Please install it first", tool.Command)
	}

	fmt.Printf("Launching %s...\n", tool.Name)

	cmd := exec.Command(tool.Command, args...)
	cmd.Env = append(os.Environ(), toolEnv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
