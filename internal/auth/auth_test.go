package auth

import (
	"testing"

	"github.com/dominionthedev/ollacloud/internal/config"
)

func TestResolve_FlagTakesPriority(t *testing.T) {
	t.Setenv("OLLACLOUD_API_KEY", "env-key")
	cfg := config.Resolved{APIKey: "config-key"}

	key, err := Resolve(ResolveOptions{FlagValue: "flag-key"}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "flag-key" {
		t.Errorf("expected flag-key, got %q", key)
	}
}

func TestResolve_EnvOverridesConfig(t *testing.T) {
	// Use OLLACLOUD_API_KEY — takes priority over config file value.
	t.Setenv("OLLACLOUD_API_KEY", "env-key")
	t.Setenv("OLLAMA_API_KEY", "")
	cfg := config.Resolved{APIKey: "config-key"}

	key, err := Resolve(ResolveOptions{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-key" {
		t.Errorf("expected env-key, got %q", key)
	}
}

func TestResolve_OllamaEnvCompat(t *testing.T) {
	// OLLAMA_API_KEY should also work (drop-in compat).
	t.Setenv("OLLACLOUD_API_KEY", "")
	t.Setenv("OLLAMA_API_KEY", "ollama-env-key")
	cfg := config.Resolved{APIKey: "config-key"}

	key, err := Resolve(ResolveOptions{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "ollama-env-key" {
		t.Errorf("expected ollama-env-key, got %q", key)
	}
}

func TestResolve_FallsBackToConfig(t *testing.T) {
	t.Setenv("OLLACLOUD_API_KEY", "")
	t.Setenv("OLLAMA_API_KEY", "")
	cfg := config.Resolved{APIKey: "config-key"}

	key, err := Resolve(ResolveOptions{}, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "config-key" {
		t.Errorf("expected config-key, got %q", key)
	}
}

func TestResolve_NoKeyNoPrompt(t *testing.T) {
	t.Setenv("OLLACLOUD_API_KEY", "")
	t.Setenv("OLLAMA_API_KEY", "")
	cfg := config.Resolved{}

	_, err := Resolve(ResolveOptions{AllowPrompt: false}, cfg)
	if err == nil {
		t.Fatal("expected ErrNoKey, got nil")
	}
	if err != ErrNoKey {
		t.Errorf("expected ErrNoKey, got %v", err)
	}
}

func TestResolve_TrimsWhitespace(t *testing.T) {
	t.Setenv("OLLACLOUD_API_KEY", "  spaced-key  ")
	t.Setenv("OLLAMA_API_KEY", "")

	key, err := Resolve(ResolveOptions{}, config.Resolved{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "spaced-key" {
		t.Errorf("expected trimmed key, got %q", key)
	}
}

func TestValidate_RejectsEmpty(t *testing.T) {
	for _, c := range []string{"", "   ", "\t\n"} {
		if err := Validate(c); err == nil {
			t.Errorf("Validate(%q) should return error", c)
		}
	}
}

func TestValidate_AcceptsNonEmpty(t *testing.T) {
	for _, c := range []string{"sk-abc123", "ollama_key_xyz", "any-non-empty-string"} {
		if err := Validate(c); err != nil {
			t.Errorf("Validate(%q) unexpected error: %v", c, err)
		}
	}
}
