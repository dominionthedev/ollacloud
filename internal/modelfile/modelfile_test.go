package modelfile

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	input := `
FROM llama3
# Comment
PARAMETER temperature 0.7
PARAMETER top_p 0.9
SYSTEM """
You are a helpful assistant.
"""
TEMPLATE "{{ .System }}\nUSER: {{ .Prompt }}\nASSISTANT:"
MESSAGE user "Hello"
MESSAGE assistant "Hi there!"
LICENSE MIT
LICENSE "GPL v3"
`
	req, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if req.From != "llama3" {
		t.Errorf("expected From llama3, got %q", req.From)
	}

	if req.Parameters["temperature"] != 0.7 {
		t.Errorf("expected temperature 0.7, got %v (%T)", req.Parameters["temperature"], req.Parameters["temperature"])
	}

	if req.Parameters["top_p"] != 0.9 {
		t.Errorf("expected top_p 0.9, got %v (%T)", req.Parameters["top_p"], req.Parameters["top_p"])
	}

	if req.System != "\nYou are a helpful assistant.\n" {
		t.Errorf("expected system prompt, got %q", req.System)
	}

	if req.Template != `{{ .System }}\nUSER: {{ .Prompt }}\nASSISTANT:` {
		t.Errorf("expected template, got %q", req.Template)
	}

	if len(req.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(req.Messages))
	} else {
		if req.Messages[0].Role != "user" || req.Messages[0].Content != "Hello" {
			t.Errorf("unexpected first message: %+v", req.Messages[0])
		}
	}

	licenses, ok := req.License.([]string)
	if !ok || len(licenses) != 2 || licenses[0] != "MIT" || licenses[1] != "GPL v3" {
		t.Errorf("expected 2 licenses, got %v", req.License)
	}
}
