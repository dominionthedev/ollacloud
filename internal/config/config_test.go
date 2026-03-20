package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── File round-trip ──────────────────────────────────────────────────────────

func TestLoadFile_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	// Clear env vars so they don't pollute the resolved values.
	t.Setenv("OLLACLOUD_API_KEY", "")
	t.Setenv("OLLAMA_API_KEY", "")

	f, err := LoadFile()
	if err != nil {
		t.Fatalf("LoadFile() with missing file: %v", err)
	}
	// Zero value — nothing persisted yet.
	if f.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", f.APIKey)
	}
}

func TestSaveFileAndLoadFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	want := File{
		APIKey:      "sk-test-key",
		UpstreamURL: "https://custom.ollama.com",
		Host:        "0.0.0.0:12345",
	}

	if err := SaveFile(want); err != nil {
		t.Fatalf("SaveFile(): %v", err)
	}

	got, err := LoadFile()
	if err != nil {
		t.Fatalf("LoadFile(): %v", err)
	}

	if got.APIKey != want.APIKey {
		t.Errorf("APIKey: got %q, want %q", got.APIKey, want.APIKey)
	}
	if got.UpstreamURL != want.UpstreamURL {
		t.Errorf("UpstreamURL: got %q, want %q", got.UpstreamURL, want.UpstreamURL)
	}
	if got.Host != want.Host {
		t.Errorf("Host: got %q, want %q", got.Host, want.Host)
	}
}

func TestSaveFile_CreatesDir(t *testing.T) {
	nested := filepath.Join(t.TempDir(), "deep", "nested")
	t.Setenv("XDG_CONFIG_HOME", nested)

	if err := SaveFile(File{APIKey: "testkey"}); err != nil {
		t.Fatalf("SaveFile() failed to create dirs: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatalf("Path(): %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("config file was not created at %s", path)
	}
}

func TestSaveFile_FilePermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := SaveFile(File{}); err != nil {
		t.Fatalf("SaveFile(): %v", err)
	}

	path, _ := Path()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(): %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

// ─── Resolved (env overlay) ───────────────────────────────────────────────────

func TestLoad_ReturnsDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("OLLACLOUD_API_KEY", "")
	t.Setenv("OLLAMA_API_KEY", "")
	t.Setenv("OLLACLOUD_HOST", "")
	t.Setenv("OLLAMA_HOST", "")
	t.Setenv("OLLACLOUD_UPSTREAM", "")

	r, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if r.UpstreamURL != "https://ollama.com" {
		t.Errorf("default UpstreamURL = %q, want https://ollama.com", r.UpstreamURL)
	}
	if r.Host != "127.0.0.1:11434" {
		t.Errorf("default Host = %q, want 127.0.0.1:11434", r.Host)
	}
}

func TestLoad_EnvVarOverridesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Write config file with one value.
	SaveFile(File{APIKey: "file-key"}) //nolint:errcheck

	// Set env var to override it.
	t.Setenv("OLLACLOUD_API_KEY", "env-key")
	t.Setenv("OLLAMA_API_KEY", "")

	r, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if r.APIKey != "env-key" {
		t.Errorf("env var should override file: got %q, want env-key", r.APIKey)
	}
}

func TestLoad_OllamaEnvCompat(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("OLLACLOUD_API_KEY", "")
	t.Setenv("OLLAMA_API_KEY", "ollama-key")

	r, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if r.APIKey != "ollama-key" {
		t.Errorf("OLLAMA_API_KEY compat: got %q, want ollama-key", r.APIKey)
	}
}

// ─── Path helpers ─────────────────────────────────────────────────────────────

func TestPath_ContainsOllacloud(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	path, err := Path()
	if err != nil {
		t.Fatalf("Path(): %v", err)
	}
	if filepath.Base(filepath.Dir(path)) != "ollacloud" {
		t.Errorf("config dir should be 'ollacloud', got path %q", path)
	}
	if filepath.Base(path) != "config.toml" {
		t.Errorf("config file should be 'config.toml', got %q", filepath.Base(path))
	}
}
