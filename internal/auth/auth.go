// Package auth resolves the Ollama Cloud API key from multiple sources
// in a well-defined priority order:
//
//  1. Explicit --key flag value (highest priority)
//  2. OLLACLOUD_API_KEY environment variable  }  handled together
//  3. OLLAMA_API_KEY environment variable     }  by env.APIKey()
//  4. Config file (~/.config/ollacloud/config.toml)
//  5. Interactive stdin prompt (lowest — interactive contexts only)
package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/dominionthedev/ollacloud/internal/config"
	"github.com/dominionthedev/ollacloud/internal/env"
)

// ErrNoKey is returned when no API key can be resolved from any source.
var ErrNoKey = fmt.Errorf("no API key found — run `ollacloud auth set` or set OLLACLOUD_API_KEY")

// ResolveOptions controls which resolution methods are attempted.
type ResolveOptions struct {
	// FlagValue is a key passed directly via --key / -k.
	// Empty string means the flag was not set.
	FlagValue string

	// AllowPrompt allows falling back to an interactive stdin prompt.
	// Must be false in non-interactive contexts (daemon, API handlers).
	AllowPrompt bool
}

// Resolve returns the best available API key according to the priority chain.
// cfg is the already-resolved config so we don't re-read disk on every call.
func Resolve(opts ResolveOptions, cfg config.Resolved) (string, error) {
	// 1. Explicit --key flag
	if k := strings.TrimSpace(opts.FlagValue); k != "" {
		return k, nil
	}

	// 2+3. OLLACLOUD_API_KEY then OLLAMA_API_KEY (env.APIKey handles both)
	if k := env.APIKey(); k != "" {
		return k, nil
	}

	// 4. Config file (already overlaid into cfg by config.Load)
	if k := strings.TrimSpace(cfg.APIKey); k != "" {
		return k, nil
	}

	// 5. Interactive prompt
	if opts.AllowPrompt {
		return promptKey()
	}

	return "", ErrNoKey
}

// promptKey reads an API key from stdin interactively.
func promptKey() (string, error) {
	fmt.Fprint(os.Stderr, "Enter Ollama Cloud API key: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("reading key from stdin: %w", err)
		}
		return "", fmt.Errorf("no input received")
	}
	key := strings.TrimSpace(scanner.Text())
	if key == "" {
		return "", fmt.Errorf("empty API key")
	}
	return key, nil
}

// Validate returns an error if key is blank.
func Validate(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("API key must not be blank")
	}
	return nil
}
