// Package config manages ollacloud persistent configuration stored at
// ~/.config/ollacloud/config.toml.
//
// Resolution order (highest to lowest priority):
//
//  1. Environment variables  (OLLACLOUD_* or OLLAMA_* — see internal/env)
//  2. Config file            (~/.config/ollacloud/config.toml)
//  3. Built-in defaults
//
// The config file is written only when the user explicitly runs
// `ollacloud auth set` or `ollacloud config set`. Runtime env vars are
// never written to disk.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/dominionthedev/ollacloud/internal/env"
)

const (
	dirName  = "ollacloud"
	fileName = "config.toml"
)

// File holds the fields that are persisted to disk. Only user-chosen
// overrides live here; runtime-only settings live in env vars.
type File struct {
	// APIKey stored by `ollacloud auth set`. Lowest priority — env var wins.
	APIKey string `toml:"api_key"`

	// UpstreamURL overrides the default https://ollama.com.
	UpstreamURL string `toml:"upstream_url"`

	// Host overrides the default 127.0.0.1:11434 bind address.
	Host string `toml:"host"`
}

// Resolved is the final merged configuration after applying env vars over
// config file values over defaults. This is what the rest of the app uses.
type Resolved struct {
	APIKey      string
	UpstreamURL string
	Host        string
}

// Load reads the config file (if it exists), then overlays env vars on top,
// and returns the fully resolved configuration.
func Load() (Resolved, error) {
	f, err := loadFile()
	if err != nil {
		return Resolved{}, err
	}
	return resolve(f), nil
}

// resolve merges env vars over config file values over defaults.
// This is the single place where priority is enforced.
func resolve(f File) Resolved {
	r := Resolved{
		// Start with config file values (which already incorporate defaults
		// via the zero value of File + our fallback logic below).
		APIKey:      f.APIKey,
		UpstreamURL: f.UpstreamURL,
		Host:        f.Host,
	}

	// Apply built-in defaults for anything not in the config file.
	if r.UpstreamURL == "" {
		r.UpstreamURL = "https://ollama.com"
	}
	if r.Host == "" {
		r.Host = "127.0.0.1:11434"
	}

	// Env vars override everything — this is where OLLACLOUD_* / OLLAMA_* win.
	if k := env.APIKey(); k != "" {
		r.APIKey = k
	}
	if u := env.UpstreamURL(); u != "https://ollama.com" {
		// Only override if the user actually set the env var
		// (env.UpstreamURL always returns a non-empty value, so we check
		// whether it differs from the hard default).
		r.UpstreamURL = u
	}
	if h := env.Host(); h != "127.0.0.1:11434" {
		r.Host = h
	}

	return r
}

// ─── File I/O ─────────────────────────────────────────────────────────────────

func loadFile() (File, error) {
	path, err := Path()
	if err != nil {
		return File{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, nil // first run — no file is fine
		}
		return File{}, fmt.Errorf("config: read %s: %w", path, err)
	}

	var f File
	if _, err := toml.Decode(string(data), &f); err != nil {
		return File{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return f, nil
}

// SaveFile writes a File to disk. Used only by `ollacloud auth set` and
// `ollacloud config set`. Never called during normal daemon operation.
func SaveFile(f File) error {
	path, err := Path()
	if err != nil {
		return err
	}

	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: open %s: %w", path, err)
	}
	defer fh.Close()

	if err := toml.NewEncoder(fh).Encode(f); err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	return nil
}

// LoadFile returns the raw on-disk config without env var overlay.
// Used by commands that need to read/modify the file directly (auth set, etc.).
func LoadFile() (File, error) {
	return loadFile()
}

// Dir returns the config directory, creating it if needed.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: cannot find user config dir: %w", err)
	}
	dir := filepath.Join(base, dirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("config: cannot create config dir: %w", err)
	}
	return dir, nil
}

// DataDir returns the data directory, creating it if needed.
// This is used for session history, logs, and PID files.
func DataDir() (string, error) {
	// Follow XDG_DATA_HOME if set, else ~/.local/share
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: cannot find user home dir: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}

	dir := filepath.Join(base, dirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("config: cannot create data dir: %w", err)
	}
	return dir, nil
}

// Path returns the full path to the config file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}
