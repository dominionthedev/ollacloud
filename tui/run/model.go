// Package run implements the interactive Bubbletea TUI for `ollacloud run`.
// It manages a full chat session with the cloud model, handling:
//   - Streaming token delivery via a goroutine + tea.Cmd channel pump
//   - Slash command parsing (/exit, /clear, /set, /show, /help)
//   - Multiline input mode (""" ... """)
//   - Session history (messages accumulated across turns)
package run

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dominionthedev/ollacloud/internal/api"
)

// ─── Messages ─────────────────────────────────────────────────────────────────

// TokenMsg carries one streaming token from the pump goroutine to the model.
type TokenMsg struct {
	Token string
}

// DoneMsg signals the stream finished cleanly with final stats.
type DoneMsg struct {
	Stats api.GenerationStats
}

// ErrMsg carries an error from the pump goroutine to the model.
type ErrMsg struct {
	Err error
}

// ─── Session params (settable via /set) ───────────────────────────────────────

// SessionParams holds runtime generation options the user can override
// with /set parameter <key> <value> commands.
type SessionParams struct {
	Temperature *float64
	NumCtx      *int
	TopP        *float64
	System      string
}

// toOptions converts SessionParams to the map[string]any Options field.
func (sp SessionParams) toOptions() map[string]any {
	opts := make(map[string]any)
	if sp.Temperature != nil {
		opts["temperature"] = *sp.Temperature
	}
	if sp.NumCtx != nil {
		opts["num_ctx"] = *sp.NumCtx
	}
	if sp.TopP != nil {
		opts["top_p"] = *sp.TopP
	}
	return opts
}

// ─── Model ────────────────────────────────────────────────────────────────────

// State is the complete Bubbletea model for the run command.
type State struct {
	// Configuration
	modelName   string
	daemonHost  string

	// UI components
	viewport viewport.Model
	textarea textarea.Model
	ready    bool // true after WindowSizeMsg received

	// Chat state
	history       []api.Message // full conversation history
	currentAssist string        // tokens accumulated for the current assistant turn
	streaming     bool          // true while a generation is in flight

	// Multiline input state
	multiline     bool   // true when inside a """ block
	multilineBuf  string // accumulated lines between """

	// Session overrides
	params SessionParams

	// Display buffer — rendered text shown in the viewport
	displayBuf string

	// Width/height from last WindowSizeMsg
	width  int
	height int

	// Error state
	lastErr error

	// pumpCh receives items from the streaming goroutine.
	// Stored on State so waitForToken can reference it across Update calls.
	pumpCh <-chan pumpItem
}

// New creates a fresh State for the given model and daemon port.
func New(modelName string, daemonHost string) State {
	ta := textarea.New()
	ta.Placeholder = "Send a message… (/help for commands)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // unlimited

	return State{
		modelName:  modelName,
		daemonHost: daemonHost,
		textarea:   ta,
	}
}

// Init satisfies tea.Model.
func (s State) Init() tea.Cmd {
	return textarea.Blink
}
