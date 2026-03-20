// Package proxy contains the core HTTP forwarding and NDJSON streaming logic.
package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// StreamLine is one raw NDJSON line delivered from the upstream to a caller.
// Exactly one of Line or Err will be non-zero. Done=true signals the stream
// ended cleanly (Err will be nil in that case).
type StreamLine struct {
	Line []byte
	Err  error
	Done bool
}

// Pump reads the upstream response body line by line and sends each raw
// NDJSON line to out. It always closes out when it returns.
//
// Context cancellation is fully respected: a separate goroutine closes the
// body as soon as ctx.Done() fires, which unblocks any in-progress Read
// inside bufio.Scanner and causes it to return an error. Pump then exits
// cleanly and sends an ErrMsg to out.
//
// The goroutine is started by Forward — callers should not call Pump directly.
func Pump(ctx context.Context, body io.ReadCloser, out chan<- StreamLine) {
	defer close(out)
	defer body.Close()

	// Watch for context cancellation and close the body to unblock the scanner.
	// Without this, bufio.Scanner.Scan() blocks in body.Read() indefinitely
	// even after the context is cancelled.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			body.Close() // unblocks the scanner
		case <-done:
			// Pump exited normally — nothing to do.
		}
	}()

	scanner := bufio.NewScanner(body)
	// 4 MB line buffer — large enough for embed responses or verbose show output.
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	for scanner.Scan() {
		line := make([]byte, len(scanner.Bytes()))
		copy(line, scanner.Bytes())

		if len(line) == 0 {
			continue // skip blank lines
		}

		select {
		case <-ctx.Done():
			out <- StreamLine{Err: ctx.Err()}
			return
		case out <- StreamLine{Line: line}:
		}
	}

	// Scanner stopped — either EOF (clean) or an error (including the body
	// being closed by the context watcher above).
	if err := scanner.Err(); err != nil {
		// If the context was cancelled, report that as the root cause
		// rather than the low-level I/O error from closing the body.
		if ctx.Err() != nil {
			select {
			case out <- StreamLine{Err: ctx.Err()}:
			default:
			}
			return
		}
		select {
		case out <- StreamLine{Err: fmt.Errorf("stream read: %w", err)}:
		default:
		}
		return
	}

	// Clean EOF — signal completion.
	select {
	case <-ctx.Done():
		out <- StreamLine{Err: ctx.Err()}
	case out <- StreamLine{Done: true}:
	}
}

// WriteStream reads from lines and writes each one verbatim to w, flushing
// after every line. It also watches for Ollama-style mid-stream error objects
// embedded in the NDJSON and returns them as Go errors.
//
// w must implement http.Flusher. If it does not, responses will still be
// correct but will not stream incrementally.
func WriteStream(w http.ResponseWriter, lines <-chan StreamLine) error {
	flusher, canFlush := w.(http.Flusher)

	for sl := range lines {
		if sl.Err != nil {
			return sl.Err
		}
		if sl.Done {
			break
		}

		// Check for a mid-stream error embedded in the NDJSON payload.
		// Ollama embeds {"error":"..."} as a normal line without changing
		// the HTTP status, so we must inspect the line content.
		var maybeErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(sl.Line, &maybeErr) == nil && maybeErr.Error != "" {
			// Write the error line so the client receives it, then return.
			w.Write(sl.Line) //nolint:errcheck
			w.Write([]byte("\n"))
			if canFlush {
				flusher.Flush()
			}
			return fmt.Errorf("upstream error: %s", maybeErr.Error)
		}

		if _, err := w.Write(sl.Line); err != nil {
			return fmt.Errorf("write to client: %w", err)
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return fmt.Errorf("write newline: %w", err)
		}
		if canFlush {
			flusher.Flush()
		}
	}

	return nil
}
