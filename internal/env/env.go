// Package env resolves ollacloud runtime configuration from environment
// variables. It follows Ollama's own pattern exactly:
//
//   - OLLACLOUD_* variables take priority (ollacloud-specific)
//   - OLLAMA_* variables are honoured as fallbacks for drop-in compatibility
//   - Sensible defaults apply when neither is set
//
// Every variable is readable via `ollacloud serve --help`, which prints the
// full env table just like `ollama serve --help` does.
package env

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Var holds the resolved value and metadata for a single env variable.
type Var struct {
	Name        string
	Value       string
	Description string
}

// APIKey returns the Ollama Cloud API key.
// Priority: OLLACLOUD_API_KEY > OLLAMA_API_KEY
func APIKey() string {
	return firstNonEmpty("OLLACLOUD_API_KEY", "OLLAMA_API_KEY")
}

// Host returns the host:port the daemon binds to.
// Priority: OLLACLOUD_HOST > OLLAMA_HOST > 127.0.0.1:11434
func Host() string {
	raw := firstNonEmpty("OLLACLOUD_HOST", "OLLAMA_HOST")
	if raw == "" {
		return "127.0.0.1:11434"
	}
	return normaliseHost(raw)
}

// UpstreamURL returns the Ollama Cloud base URL.
// Priority: OLLACLOUD_UPSTREAM > https://ollama.com
func UpstreamURL() string {
	if v := os.Getenv("OLLACLOUD_UPSTREAM"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://ollama.com"
}

// Origins returns allowed CORS origins.
// Priority: OLLACLOUD_ORIGINS > OLLAMA_ORIGINS > built-in safe list
func Origins() []string {
	raw := firstNonEmpty("OLLACLOUD_ORIGINS", "OLLAMA_ORIGINS")
	var origins []string
	if raw != "" {
		origins = strings.Split(raw, ",")
	}
	for _, h := range []string{"localhost", "127.0.0.1", "0.0.0.0"} {
		origins = append(origins,
			"http://"+h,
			"https://"+h,
			"http://"+net.JoinHostPort(h, "*"),
			"https://"+net.JoinHostPort(h, "*"),
		)
	}
	origins = append(origins,
		"app://*", "file://*", "tauri://*",
		"vscode-webview://*", "vscode-file://*",
	)
	return origins
}

// KeepAlive returns how long a model is considered active after its last request.
// Priority: OLLACLOUD_KEEP_ALIVE > OLLAMA_KEEP_ALIVE > 5m
func KeepAlive() time.Duration {
	raw := firstNonEmpty("OLLACLOUD_KEEP_ALIVE", "OLLAMA_KEEP_ALIVE")
	if raw == "" {
		return 5 * time.Minute
	}
	d, err := parseDuration(raw)
	if err != nil {
		slog.Warn("invalid keep-alive value, using default 5m", "value", raw, "err", err)
		return 5 * time.Minute
	}
	if d < 0 {
		return 0
	}
	return d
}

// Debug returns true when debug logging is enabled.
// Priority: OLLACLOUD_DEBUG > OLLAMA_DEBUG
func Debug() bool {
	return parseBool(firstNonEmpty("OLLACLOUD_DEBUG", "OLLAMA_DEBUG"))
}

// MaxQueue returns the maximum number of queued requests before 503.
// Priority: OLLACLOUD_MAX_QUEUE > OLLAMA_MAX_QUEUE > 512
func MaxQueue() int {
	raw := firstNonEmpty("OLLACLOUD_MAX_QUEUE", "OLLAMA_MAX_QUEUE")
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return n
	}
	return 512
}

// Table returns all env vars with their current resolved values for display.
func Table() []Var {
	return []Var{
		{"OLLACLOUD_API_KEY", mask(APIKey()), "Ollama Cloud API key (also accepts OLLAMA_API_KEY)"},
		{"OLLACLOUD_HOST", Host(), "Bind address for the daemon (also accepts OLLAMA_HOST)"},
		{"OLLACLOUD_UPSTREAM", UpstreamURL(), "Ollama Cloud base URL"},
		{"OLLACLOUD_ORIGINS", strings.Join(Origins(), ","), "Allowed CORS origins (also accepts OLLAMA_ORIGINS)"},
		{"OLLACLOUD_KEEP_ALIVE", KeepAlive().String(), "Active model expiry window (also accepts OLLAMA_KEEP_ALIVE)"},
		{"OLLACLOUD_MAX_QUEUE", fmt.Sprintf("%d", MaxQueue()), "Max queued requests before 503 (also accepts OLLAMA_MAX_QUEUE)"},
		{"OLLACLOUD_DEBUG", fmt.Sprintf("%v", Debug()), "Enable debug logging (also accepts OLLAMA_DEBUG)"},
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func firstNonEmpty(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func normaliseHost(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	host, port, err := net.SplitHostPort(raw)
	if err != nil {
		host = raw
		port = "11434"
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "11434"
	}
	return net.JoinHostPort(host, port)
}

func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(n * float64(time.Second)), nil
	}
	return 0, fmt.Errorf("cannot parse duration %q", s)
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func mask(key string) string {
	if key == "" {
		return "(not set)"
	}
	if len(key) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}
