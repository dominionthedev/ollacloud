package modelfile

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dominionthedev/ollacloud/internal/api"
)

// Parse parses a Modelfile and returns a CreateRequest.
func Parse(r io.Reader) (*api.CreateRequest, error) {
	req := &api.CreateRequest{
		Parameters: make(map[string]any),
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		parts := strings.SplitN(trimmed, " ", 2)
		if len(parts) < 2 {
			continue
		}

		command := strings.ToUpper(parts[0])
		args := strings.TrimSpace(parts[1])

		// Handle multiline arguments starting with """
		if strings.HasPrefix(args, `"""`) {
			if strings.HasSuffix(args, `"""`) && len(args) >= 6 {
				args = args[3 : len(args)-3]
			} else {
				var sb strings.Builder
				sb.WriteString(strings.TrimPrefix(args, `"""`))
				sb.WriteString("\n")
				for scanner.Scan() {
					nextLine := scanner.Text()
					if strings.HasSuffix(nextLine, `"""`) {
						sb.WriteString(strings.TrimSuffix(nextLine, `"""`))
						break
					}
					sb.WriteString(nextLine)
					sb.WriteString("\n")
				}
				args = sb.String()
			}
		} else {
			args = unquote(args)
		}

		switch command {
		case "FROM":
			req.From = args
		case "TEMPLATE":
			req.Template = args
		case "SYSTEM":
			req.System = args
		case "PARAMETER":
			paramParts := strings.SplitN(args, " ", 2)
			if len(paramParts) == 2 {
				req.Parameters[paramParts[0]] = parseParamValue(paramParts[1])
			}
		case "MESSAGE":
			msgParts := strings.SplitN(args, " ", 2)
			if len(msgParts) == 2 {
				req.Messages = append(req.Messages, api.Message{
					Role:    msgParts[0],
					Content: unquote(msgParts[1]),
				})
			}
		case "LICENSE":
			if req.License == nil {
				req.License = unquote(args)
			} else {
				switch v := req.License.(type) {
				case string:
					req.License = []string{v, unquote(args)}
				case []string:
					req.License = append(v, unquote(args))
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if req.From == "" {
		return nil, fmt.Errorf("Modelfile must start with FROM")
	}

	return req, nil
}

func unquote(s string) string {
	if len(s) >= 6 && strings.HasPrefix(s, `"""`) && strings.HasSuffix(s, `"""`) {
		return s[3 : len(s)-3]
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func parseParamValue(s string) any {
	s = unquote(s)
	// Try to parse as int
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	// Try to parse as float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// Try to parse as bool
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return s
}
