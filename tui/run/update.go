package run

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dominionthedev/ollacloud/internal/api"
)

// ─── Pump channel message ─────────────────────────────────────────────────────
// pumpItem is one item read from the scanner goroutine's channel.
// It is internal to the streaming machinery.
type pumpItem struct {
	line []byte
	done bool
	err  error
}

// waitForToken returns a Cmd that reads one item from ch and converts it to a
// Bubbletea message. After receiving a TokenMsg the Update loop returns another
// waitForToken to continue draining the channel — this is the canonical
// Bubbletea pattern for continuous streaming.
func waitForToken(ch <-chan pumpItem) tea.Cmd {
	return func() tea.Msg {
		item, ok := <-ch
		if !ok {
			return DoneMsg{}
		}
		if item.err != nil {
			return ErrMsg{Err: item.err}
		}
		if item.done {
			return DoneMsg{}
		}

		// Try to decode as a chat response frame.
		var frame api.ChatResponse
		if err := json.Unmarshal(item.line, &frame); err == nil {
			if frame.Message.Content != "" {
				return TokenMsg{Token: frame.Message.Content}
			}
			if frame.Done {
				return DoneMsg{Stats: frame.GenerationStats}
			}
		}

		// Check for embedded error.
		var errFrame api.ErrorResponse
		if json.Unmarshal(item.line, &errFrame) == nil && errFrame.Error != "" {
			return ErrMsg{Err: fmt.Errorf(errFrame.Error)}
		}

		// Unknown frame — skip, fetch next.
		return waitForToken(ch)()
	}
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (s State) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Window resize ─────────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		headerH := 2
		inputH := 5
		footerH := 1
		vpH := msg.Height - headerH - inputH - footerH
		if vpH < 4 {
			vpH = 4
		}
		s.textarea.SetWidth(msg.Width - 2)
		if !s.ready {
			s.viewport.Width = msg.Width
			s.viewport.Height = vpH
			s.viewport.SetContent(s.displayBuf)
			s.ready = true
		} else {
			s.viewport.Width = msg.Width
			s.viewport.Height = vpH
		}
		return s, nil

	// ── Streaming token ───────────────────────────────────────────────────────
	case TokenMsg:
		s.currentAssist += msg.Token
		s.displayBuf += msg.Token
		s.viewport.SetContent(s.displayBuf)
		s.viewport.GotoBottom()
		// Return another waitForToken to keep draining the pump channel.
		// The channel is stored in s.pumpCh — see sendMessage.
		return s, waitForToken(s.pumpCh)

	// ── Generation done ───────────────────────────────────────────────────────
	case DoneMsg:
		s.streaming = false
		if s.currentAssist != "" {
			s.history = append(s.history, api.Message{
				Role:    "assistant",
				Content: s.currentAssist,
			})
			s.currentAssist = ""
		}
		s.displayBuf += "\n\n"
		s.viewport.SetContent(s.displayBuf)
		s.viewport.GotoBottom()
		return s, nil

	// ── Error from pump ───────────────────────────────────────────────────────
	case ErrMsg:
		s.streaming = false
		s.lastErr = msg.Err
		s.displayBuf += fmt.Sprintf("\n\n⚠  Error: %v\n\n", msg.Err)
		s.viewport.SetContent(s.displayBuf)
		s.viewport.GotoBottom()
		return s, nil

	// ── Keyboard input ────────────────────────────────────────────────────────
	case tea.KeyMsg:
		switch msg.Type {

		case tea.KeyCtrlC:
			return s, tea.Quit

		case tea.KeyEsc:
			if !s.streaming {
				return s, tea.Quit
			}
			return s, nil

		case tea.KeyEnter:
			// Alt+Enter → newline in textarea.
			if msg.Alt {
				var cmd tea.Cmd
				s.textarea, cmd = s.textarea.Update(msg)
				return s, cmd
			}
			if s.streaming {
				return s, nil
			}
			input := strings.TrimSpace(s.textarea.Value())
			s.textarea.Reset()
			if input == "" {
				return s, nil
			}
			return s.handleInput(input)

		case tea.KeyPgUp:
			s.viewport.HalfViewUp()
			return s, nil

		case tea.KeyPgDown:
			s.viewport.HalfViewDown()
			return s, nil
		}
	}

	// Pass all other events to the textarea.
	var cmd tea.Cmd
	s.textarea, cmd = s.textarea.Update(msg)
	return s, cmd
}

// handleInput routes user input to slash commands or chat.
func (s State) handleInput(input string) (tea.Model, tea.Cmd) {
	// Multiline end marker.
	if s.multiline {
		if strings.TrimSpace(input) == `"""` {
			s.multiline = false
			full := s.multilineBuf
			s.multilineBuf = ""
			return s.sendMessage(full)
		}
		s.multilineBuf += input + "\n"
		return s, nil
	}
	// Multiline start marker.
	if strings.TrimSpace(input) == `"""` {
		s.multiline = true
		s.displayBuf += `"""` + "\n"
		s.viewport.SetContent(s.displayBuf)
		return s, nil
	}
	// Slash command.
	if strings.HasPrefix(input, "/") {
		return s.handleSlash(input)
	}
	return s.sendMessage(input)
}

// handleSlash executes an in-session slash command.
func (s State) handleSlash(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return s, nil
	}

	switch parts[0] {
	case "/exit", "/bye", "/quit":
		return s, tea.Quit

	case "/clear":
		s.history = nil
		s.displayBuf = ""
		s.viewport.SetContent("")
		return s, nil

	case "/help":
		s.displayBuf += helpText()
		s.viewport.SetContent(s.displayBuf)
		s.viewport.GotoBottom()
		return s, nil

	case "/show":
		if len(parts) >= 2 {
			switch parts[1] {
			case "info":
				s.displayBuf += fmt.Sprintf("Model: %s\nHost:  %s\n\n", s.modelName, s.daemonHost)
			case "parameters":
				s.displayBuf += formatParams(s.params)
			}
			s.viewport.SetContent(s.displayBuf)
			s.viewport.GotoBottom()
		}
		return s, nil

	case "/set":
		if len(parts) < 3 {
			s.displayBuf += "Usage: /set system <text>  or  /set parameter <key> <value>\n\n"
			s.viewport.SetContent(s.displayBuf)
			return s, nil
		}
		switch parts[1] {
		case "system":
			s.params.System = strings.Join(parts[2:], " ")
			s.displayBuf += "System prompt updated.\n\n"
		case "parameter":
			if len(parts) < 4 {
				s.displayBuf += "Usage: /set parameter <key> <value>\n\n"
			} else {
				s = applyParam(s, parts[2], parts[3])
				s.displayBuf += fmt.Sprintf("Set %s = %s\n\n", parts[2], parts[3])
			}
		}
		s.viewport.SetContent(s.displayBuf)
		s.viewport.GotoBottom()
		return s, nil
	}

	s.displayBuf += fmt.Sprintf("Unknown command: %s  (try /help)\n\n", parts[0])
	s.viewport.SetContent(s.displayBuf)
	s.viewport.GotoBottom()
	return s, nil
}

// sendMessage appends the user turn, starts the streaming goroutine via a
// buffered channel, and returns the first waitForToken cmd.
func (s State) sendMessage(text string) (tea.Model, tea.Cmd) {
	s.displayBuf += ">>> " + text + "\n\n"
	s.viewport.SetContent(s.displayBuf)
	s.viewport.GotoBottom()

	userMsg := api.Message{Role: "user", Content: text}

	msgs := make([]api.Message, 0, len(s.history)+2)
	if s.params.System != "" {
		msgs = append(msgs, api.Message{Role: "system", Content: s.params.System})
	}
	msgs = append(msgs, s.history...)
	msgs = append(msgs, userMsg)

	s.history = append(s.history, userMsg)
	s.streaming = true

	chatReq := api.ChatRequest{
		Model:    s.modelName,
		Messages: msgs,
		Options:  s.params.toOptions(),
	}

	body, _ := json.Marshal(chatReq)
	host := s.daemonHost

	// Buffered channel so the pump goroutine never blocks on a slow UI.
	ch := make(chan pumpItem, 128)
	s.pumpCh = ch

	// Launch the pump goroutine. It owns the HTTP response body.
	go func() {
		defer close(ch)

		resp, err := http.Post(
			fmt.Sprintf("http://%s/api/chat", host),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			ch <- pumpItem{err: fmt.Errorf("chat request: %w", err)}
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

		for scanner.Scan() {
			line := make([]byte, len(scanner.Bytes()))
			copy(line, scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			ch <- pumpItem{line: line}
		}

		if err := scanner.Err(); err != nil {
			ch <- pumpItem{err: fmt.Errorf("stream read: %w", err)}
			return
		}
		ch <- pumpItem{done: true}
	}()

	// Return the first read from the channel as the initial Cmd.
	return s, waitForToken(ch)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func helpText() string {
	return `
Commands:
  /exit, /bye        Exit the session
  /clear             Clear the conversation
  /set system <text> Set the system prompt
  /set parameter temperature <f>
  /set parameter num_ctx <n>
  /set parameter top_p <f>
  /show info         Show model and connection info
  /show parameters   Show current session parameters
  """                Toggle multiline input mode
  PgUp / PgDn        Scroll conversation history
  Ctrl+C / Esc       Quit

`
}

func formatParams(p SessionParams) string {
	var b strings.Builder
	b.WriteString("Session parameters:\n")
	if p.Temperature != nil {
		b.WriteString(fmt.Sprintf("  temperature  %.2f\n", *p.Temperature))
	}
	if p.NumCtx != nil {
		b.WriteString(fmt.Sprintf("  num_ctx      %d\n", *p.NumCtx))
	}
	if p.TopP != nil {
		b.WriteString(fmt.Sprintf("  top_p        %.2f\n", *p.TopP))
	}
	if p.System != "" {
		b.WriteString(fmt.Sprintf("  system       %q\n", p.System))
	}
	b.WriteString("\n")
	return b.String()
}

func applyParam(s State, key, val string) State {
	switch key {
	case "temperature":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			s.params.Temperature = &f
		}
	case "num_ctx":
		if n, err := strconv.Atoi(val); err == nil {
			s.params.NumCtx = &n
		}
	case "top_p":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			s.params.TopP = &f
		}
	}
	return s
}
