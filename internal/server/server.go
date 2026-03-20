// Package server runs the Ollama-compatible HTTP daemon.
// The bind address, CORS origins, upstream URL and debug level are all
// read from environment variables via the env package — no flags required.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dominionthedev/ollacloud/internal/env"
	"github.com/dominionthedev/ollacloud/internal/proxy"
	"github.com/dominionthedev/ollacloud/internal/ps"
)

// Config is the minimal runtime config passed from the CLI to the server.
// Everything else (host, origins, debug) is read from env vars directly.
type Config struct {
	// APIKey is the resolved Ollama Cloud API key.
	APIKey string

	// UpstreamURL is the Ollama Cloud base URL (from OLLACLOUD_UPSTREAM or default).
	UpstreamURL string
}

// Run starts the server and blocks until SIGINT/SIGTERM.
func Run(cfg Config) error {
	// Logging — upgrade to Debug if OLLACLOUD_DEBUG / OLLAMA_DEBUG is set.
	logLevel := slog.LevelInfo
	if env.Debug() {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	tracker := ps.New()
	fwd := proxy.New(proxy.Config{
		UpstreamBase: cfg.UpstreamURL,
		APIKey:       cfg.APIKey,
	})

	mux := http.NewServeMux()
	registerRoutes(mux, fwd, tracker)

	addr := env.Host() // e.g. "127.0.0.1:11434" or whatever OLLACLOUD_HOST says

	srv := &http.Server{
		Addr:        addr,
		Handler:     withMiddleware(mux),
		ReadTimeout: 0, // no timeout — streaming requests run arbitrarily long
		WriteTimeout: 0,
		IdleTimeout: 120 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", addr, err)
	}

	slog.Info("ollacloud daemon started",
		"addr", "http://"+addr,
		"upstream", cfg.UpstreamURL,
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("server: %w", err)
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server: shutdown: %w", err)
	}

	slog.Info("server stopped cleanly")
	return nil
}

// withMiddleware adds CORS and structured request logging to every handler.
// CORS origins are driven by OLLACLOUD_ORIGINS / OLLAMA_ORIGINS — mirroring
// Ollama's own origin policy exactly.
func withMiddleware(h http.Handler) http.Handler {
	allowedOrigins := env.Origins()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" && isAllowedOrigin(origin, allowedOrigins) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		} else if origin == "" {
			// Non-browser client (curl, SDK, etc.) — allow unconditionally.
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		start := time.Now()
		lrw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(lrw, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", lrw.status,
			"duration", time.Since(start).Round(time.Millisecond),
		)
	})
}

// isAllowedOrigin checks whether origin matches any entry in allowed.
// Entries may use glob-style wildcards in the port position (e.g. "http://localhost:*").
func isAllowedOrigin(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		// Wildcard port: "http://localhost:*" matches "http://localhost:3000"
		if strings.HasSuffix(a, ":*") {
			prefix := strings.TrimSuffix(a, ":*")
			if strings.HasPrefix(origin, prefix+":") || origin == prefix {
				return true
			}
		}
		// Wildcard scheme+host: "app://*" etc.
		if strings.HasSuffix(a, "://*") {
			scheme := strings.TrimSuffix(a, "://*")
			if strings.HasPrefix(origin, scheme+"://") {
				return true
			}
		}
		if a == origin {
			return true
		}
	}
	return false
}

// statusWriter captures the HTTP status code for logging.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}
