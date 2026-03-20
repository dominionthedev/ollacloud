package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const (
	// channelBuffer is the number of NDJSON lines we buffer in the pump channel.
	// A non-zero buffer lets the pump stay slightly ahead of the writer.
	channelBuffer = 32
)

// Config holds the dependencies the Forwarder needs.
type Config struct {
	// UpstreamBase is the Ollama Cloud base URL, e.g. "https://ollama.com".
	UpstreamBase string

	// APIKey is the Bearer token injected into every upstream request.
	APIKey string

	// HTTPClient is the http.Client used for upstream calls.
	// If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// Forwarder proxies incoming Ollama-compatible requests to Ollama Cloud,
// injecting the API key and rewriting the host.
type Forwarder struct {
	cfg    Config
	client *http.Client
}

// New creates a Forwarder from cfg.
func New(cfg Config) *Forwarder {
	c := cfg.HTTPClient
	if c == nil {
		c = http.DefaultClient
	}
	return &Forwarder{cfg: cfg, client: c}
}

// Forward proxies the incoming request r to the Ollama Cloud upstream,
// writing the response (including streaming NDJSON) directly to w.
//
// It handles both streaming and non-streaming responses transparently —
// the client's stream preference is honoured because we forward the body
// verbatim and set headers from the upstream response.
func (f *Forwarder) Forward(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Build the upstream URL by replacing the host with the cloud base.
	upstreamURL := strings.TrimRight(f.cfg.UpstreamBase, "/") + r.URL.RequestURI()

	// Read the incoming body so we can forward it.
	// For streaming endpoints this is the JSON request body, not the response.
	var bodyReader io.Reader
	if r.Body != nil {
		bodyReader = r.Body
		defer r.Body.Close()
	}

	upReq, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, bodyReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("building upstream request: %v", err))
		return
	}

	// Forward safe incoming headers.
	for _, h := range []string{"Content-Type", "Accept", "User-Agent"} {
		if v := r.Header.Get(h); v != "" {
			upReq.Header.Set(h, v)
		}
	}

	// Inject cloud authentication — this is the whole point of the proxy.
	upReq.Header.Set("Authorization", "Bearer "+f.cfg.APIKey)

	// Ensure Content-Type is set for request bodies.
	if upReq.Header.Get("Content-Type") == "" && r.Body != nil {
		upReq.Header.Set("Content-Type", "application/json")
	}

	// Execute upstream request.
	resp, err := f.client.Do(upReq)
	if err != nil {
		if ctx.Err() != nil {
			// Client disconnected — not an error worth logging loudly.
			return
		}
		slog.Error("upstream request failed", "url", upstreamURL, "err", err)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream unreachable: %v", err))
		return
	}

	// Copy upstream response headers to client response.
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Detect whether this is a streaming response.
	ct := resp.Header.Get("Content-Type")
	isStream := strings.Contains(ct, "x-ndjson") || strings.Contains(ct, "application/json") && resp.StatusCode == http.StatusOK

	if isStream && isStreamingEndpoint(r.URL.Path) {
		// Launch the pump goroutine and wire it to the response writer.
		lines := make(chan StreamLine, channelBuffer)
		go Pump(ctx, resp.Body, lines)

		if err := WriteStream(w, lines); err != nil {
			slog.Warn("stream ended with error", "path", r.URL.Path, "err", err)
		}
		return
	}

	// Non-streaming: copy body directly.
	defer resp.Body.Close()
	if _, err := io.Copy(w, resp.Body); err != nil && ctx.Err() == nil {
		slog.Warn("body copy error", "path", r.URL.Path, "err", err)
	}
}

// isStreamingEndpoint returns true for endpoints that can emit NDJSON streams.
// We use this to decide whether to run the line-by-line pump.
func isStreamingEndpoint(path string) bool {
	streaming := []string{
		"/api/generate",
		"/api/chat",
		"/api/pull",
		"/api/push",
		"/api/create",
	}
	for _, p := range streaming {
		if strings.HasSuffix(path, p) {
			return true
		}
	}
	return false
}

// writeError writes a JSON error response — used when we can't reach upstream.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}
