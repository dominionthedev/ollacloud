package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─── Pump tests ───────────────────────────────────────────────────────────────

func TestPump_DeliversLines(t *testing.T) {
	lines := "line1\nline2\nline3\n"
	body := io.NopCloser(strings.NewReader(lines))
	ch := make(chan StreamLine, 10)

	ctx := context.Background()
	go Pump(ctx, body, ch)

	var got []string
	for sl := range ch {
		if sl.Err != nil {
			t.Fatalf("unexpected error: %v", sl.Err)
		}
		if sl.Done {
			break
		}
		got = append(got, string(sl.Line))
	}

	want := []string{"line1", "line2", "line3"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d: %v", len(got), len(want), got)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("line %d: got %q, want %q", i, g, want[i])
		}
	}
}

func TestPump_SkipsBlankLines(t *testing.T) {
	body := io.NopCloser(strings.NewReader("a\n\nb\n\n"))
	ch := make(chan StreamLine, 10)
	go Pump(context.Background(), body, ch)

	var got []string
	for sl := range ch {
		if sl.Done || sl.Err != nil {
			break
		}
		got = append(got, string(sl.Line))
	}
	if len(got) != 2 {
		t.Errorf("expected 2 non-blank lines, got %d: %v", len(got), got)
	}
}

func TestPump_ContextCancellation(t *testing.T) {
	// Slow reader that blocks — cancel should unblock the pump.
	// io.PipeReader implements io.ReadCloser directly; do NOT wrap in
	// io.NopCloser because that makes Close() a no-op and breaks the test.
	pr, pw := io.Pipe()
	defer pw.Close()

	ch := make(chan StreamLine, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	go Pump(ctx, pr, ch)

	// Write one line then stall — the pump will block waiting for more data.
	pw.Write([]byte("first\n")) //nolint:errcheck

	// Drain until we see an error (context deadline) or Done.
	timeout := time.After(500 * time.Millisecond)
	for {
		select {
		case sl, ok := <-ch:
			if !ok {
				return // channel closed cleanly — also acceptable
			}
			if sl.Err != nil || sl.Done {
				return // context cancelled as expected
			}
		case <-timeout:
			t.Fatal("pump did not respect context cancellation within 500ms")
		}
	}
}

// ─── WriteStream tests ────────────────────────────────────────────────────────

func TestWriteStream_WritesLinesAndFlushes(t *testing.T) {
	ch := make(chan StreamLine, 4)
	ch <- StreamLine{Line: []byte(`{"response":"hello","done":false}`)}
	ch <- StreamLine{Line: []byte(`{"response":" world","done":true}`)}
	ch <- StreamLine{Done: true}
	close(ch)

	w := httptest.NewRecorder()
	err := WriteStream(w, ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"hello"`) {
		t.Errorf("body missing first token: %q", body)
	}
	if !strings.Contains(body, `" world"`) {
		t.Errorf("body missing second token: %q", body)
	}
}

func TestWriteStream_MidStreamError(t *testing.T) {
	ch := make(chan StreamLine, 4)
	ch <- StreamLine{Line: []byte(`{"response":"tok","done":false}`)}
	ch <- StreamLine{Line: []byte(`{"error":"model exploded"}`)}
	close(ch)

	w := httptest.NewRecorder()
	err := WriteStream(w, ch)
	if err == nil {
		t.Fatal("expected error from mid-stream error frame, got nil")
	}
	if !strings.Contains(err.Error(), "model exploded") {
		t.Errorf("error message wrong: %v", err)
	}
}

func TestWriteStream_ChannelError(t *testing.T) {
	ch := make(chan StreamLine, 2)
	ch <- StreamLine{Err: fmt.Errorf("connection reset")}
	close(ch)

	w := httptest.NewRecorder()
	err := WriteStream(w, ch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ─── Forwarder integration test ───────────────────────────────────────────────

func TestForwarder_InjectsAuthHeader(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer upstream.Close()

	fwd := New(Config{
		UpstreamBase: upstream.URL,
		APIKey:       "test-key-123",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()
	fwd.Forward(w, req)

	if gotAuth != "Bearer test-key-123" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-key-123")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestForwarder_RewritesUpstreamHost(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	fwd := New(Config{UpstreamBase: upstream.URL, APIKey: "k"})

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()
	fwd.Forward(w, req)

	if gotPath != "/api/tags" {
		t.Errorf("upstream path = %q, want /api/tags", gotPath)
	}
}

func TestForwarder_HandlesUpstreamDown(t *testing.T) {
	fwd := New(Config{
		UpstreamBase: "http://127.0.0.1:1", // nothing listening here
		APIKey:       "k",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	w := httptest.NewRecorder()
	fwd.Forward(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] == "" {
		t.Error("expected error field in JSON response")
	}
}

func TestForwarder_StreamingEndpoint(t *testing.T) {
	// Serve a multi-line NDJSON stream.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		frames := []string{
			`{"response":"Hello","done":false}`,
			`{"response":" world","done":false}`,
			`{"response":"","done":true,"done_reason":"stop"}`,
		}
		for _, f := range frames {
			fmt.Fprintln(w, f)
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	fwd := New(Config{UpstreamBase: upstream.URL, APIKey: "k"})

	body := bytes.NewBufferString(`{"model":"gemma3","prompt":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/generate", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	fwd.Forward(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	// Response body should contain all three frames.
	resp := w.Body.String()
	if !strings.Contains(resp, `"Hello"`) || !strings.Contains(resp, `"done":true`) {
		t.Errorf("streamed response missing expected content: %q", resp)
	}
}
