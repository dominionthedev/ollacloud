package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/dominionthedev/ollacloud/internal/api"
	"github.com/dominionthedev/ollacloud/internal/proxy"
	"github.com/dominionthedev/ollacloud/internal/ps"
)

// Version is returned by GET /api/version.
const Version = "1.0"

// registerRoutes wires all Ollama-compatible endpoints plus the OpenAI-compat
// /v1/* surface onto mux.
func registerRoutes(mux *http.ServeMux, fwd *proxy.Forwarder, tracker *ps.Tracker) {

	// ── Ollama native API ──────────────────────────────────────────────────────

	// Inference — streaming, track active model for /api/ps
	mux.HandleFunc("POST /api/generate", func(w http.ResponseWriter, r *http.Request) {
		model, r2 := peekModel(r)
		if model != "" {
			release := tracker.Acquire(model)
			defer release()
		}
		fwd.Forward(w, r2)
	})

	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		model, r2 := peekModel(r)
		if model != "" {
			release := tracker.Acquire(model)
			defer release()
		}
		fwd.Forward(w, r2)
	})

	// Embeddings — non-streaming
	mux.HandleFunc("POST /api/embed", fwd.Forward)

	// Legacy /api/embeddings alias (some older clients use this)
	mux.HandleFunc("POST /api/embeddings", fwd.Forward)

	// Model management
	mux.HandleFunc("GET /api/tags", fwd.Forward)
	mux.HandleFunc("POST /api/show", fwd.Forward)
	mux.HandleFunc("POST /api/create", fwd.Forward)
	mux.HandleFunc("POST /api/copy", fwd.Forward)
	mux.HandleFunc("POST /api/pull", fwd.Forward)
	mux.HandleFunc("POST /api/push", fwd.Forward)
	mux.HandleFunc("DELETE /api/delete", fwd.Forward)

	// Blob operations
	mux.HandleFunc("HEAD /api/blobs/", fwd.Forward)
	mux.HandleFunc("POST /api/blobs/", fwd.Forward)

	// Running models — synthetic response from the ps tracker
	mux.HandleFunc("GET /api/ps", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, tracker.Snapshot())
	})

	// Version — return our own version, not Ollama's
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, api.VersionResponse{Version: Version})
	})

	// ── OpenAI-compatible /v1/ surface ────────────────────────────────────────
	//
	// Ollama Cloud speaks the full OpenAI API at https://ollama.com/v1/.
	// We forward all /v1/* requests transparently — same auth injection,
	// same streaming pump. Tools like Cline, Continue, Cursor, and the
	// OpenAI Python/JS SDKs all work with base_url=http://localhost:11434/v1/.

	// Chat completions — streaming
	mux.HandleFunc("POST /v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		model, r2 := peekModel(r)
		if model != "" {
			release := tracker.Acquire(model)
			defer release()
		}
		fwd.Forward(w, r2)
	})

	// Legacy completions — streaming
	mux.HandleFunc("POST /v1/completions", fwd.Forward)

	// Embeddings
	mux.HandleFunc("POST /v1/embeddings", fwd.Forward)

	// Responses API (OpenAI Responses, added in Ollama v0.13.3)
	mux.HandleFunc("POST /v1/responses", fwd.Forward)

	// Model listing
	mux.HandleFunc("GET /v1/models", fwd.Forward)
	// Go 1.22 pattern: {model} captures everything after the slash
	mux.HandleFunc("GET /v1/models/", fwd.Forward)

	// Image generation (experimental)
	mux.HandleFunc("POST /v1/images/generations", fwd.Forward)

	// ── Health ────────────────────────────────────────────────────────────────
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ollacloud is running\n")) //nolint:errcheck
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// peekModel reads the "model" field from the JSON request body without
// consuming it, returning the model name and a new *http.Request whose
// body is fully intact for the forwarder.
//
// It uses io.TeeReader so the body bytes are captured into a buffer as
// the JSON decoder reads them, then the buffer is prepended back to the
// remaining body via io.MultiReader.
func peekModel(r *http.Request) (string, *http.Request) {
	if r.Body == nil {
		return "", r
	}

	var buf bytes.Buffer
	tee := io.TeeReader(r.Body, &buf)

	var peek struct {
		Model string `json:"model"`
	}
	json.NewDecoder(tee).Decode(&peek) //nolint:errcheck — best-effort, empty model is fine

	// Restore body: what the decoder consumed is in buf; the rest is still
	// readable from r.Body (TeeReader only reads what Decode asked for).
	r2 := r.Clone(r.Context())
	r2.Body = io.NopCloser(io.MultiReader(&buf, r.Body))
	return peek.Model, r2
}

// writeJSON encodes v as JSON and writes it to w with status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
