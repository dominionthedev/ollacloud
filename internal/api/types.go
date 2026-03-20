// Package api contains all Ollama-compatible request and response types.
// Every struct maps exactly to the Ollama REST API wire format so that
// our proxy can encode/decode without any transformation.
package api

// ─── Shared ──────────────────────────────────────────────────────────────────

// ModelDetails holds high-level metadata about a model's architecture.
type ModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ErrorResponse is returned by all endpoints on failure, and also appears
// inline in NDJSON streams when an error occurs mid-generation.
type ErrorResponse struct {
	Error string `json:"error"`
}

// VersionResponse is returned by GET /api/version.
type VersionResponse struct {
	Version string `json:"version"`
}

// ─── Generation stats (shared by Generate and Chat final frames) ──────────────

// GenerationStats holds the timing and token-count fields that appear
// only in the final streaming frame (done=true).
type GenerationStats struct {
	TotalDuration      int64 `json:"total_duration,omitempty"`
	LoadDuration       int64 `json:"load_duration,omitempty"`
	PromptEvalCount    int   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64 `json:"prompt_eval_duration,omitempty"`
	EvalCount          int   `json:"eval_count,omitempty"`
	EvalDuration       int64 `json:"eval_duration,omitempty"`
}

// ─── /api/generate ───────────────────────────────────────────────────────────

// GenerateRequest is the body for POST /api/generate.
type GenerateRequest struct {
	Model     string         `json:"model"`
	Prompt    string         `json:"prompt"`
	Suffix    string         `json:"suffix,omitempty"`
	Images    []string       `json:"images,omitempty"`
	System    string         `json:"system,omitempty"`
	Template  string         `json:"template,omitempty"`
	Stream    *bool          `json:"stream,omitempty"`
	Raw       bool           `json:"raw,omitempty"`
	Format    any            `json:"format,omitempty"`   // string | JSON schema object
	Think     any            `json:"think,omitempty"`    // bool | "high"|"medium"|"low"
	KeepAlive any            `json:"keep_alive,omitempty"` // string | int
	Options   map[string]any `json:"options,omitempty"`
	LogProbs    bool         `json:"logprobs,omitempty"`
	TopLogProbs int          `json:"top_logprobs,omitempty"`
}

// GenerateResponse is one frame in the /api/generate NDJSON stream.
// When Done=true this is the final frame and includes GenerationStats.
type GenerateResponse struct {
	Model      string `json:"model"`
	CreatedAt  string `json:"created_at"`
	Response   string `json:"response"`
	Thinking   string `json:"thinking,omitempty"`
	Done       bool   `json:"done"`
	DoneReason string `json:"done_reason,omitempty"`
	GenerationStats
}

// ─── /api/chat ───────────────────────────────────────────────────────────────

// ToolFunction describes a callable function a model may invoke.
type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Arguments   map[string]any `json:"arguments,omitempty"`
}

// ToolCall is a model's request to call a function.
type ToolCall struct {
	Function ToolFunction `json:"function"`
}

// Tool represents a function tool the model may use during chat.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// Message is a single turn in a chat conversation.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Thinking  string     `json:"thinking,omitempty"`
	Images    []string   `json:"images,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest is the body for POST /api/chat.
type ChatRequest struct {
	Model     string         `json:"model"`
	Messages  []Message      `json:"messages"`
	Stream    *bool          `json:"stream,omitempty"`
	Tools     []Tool         `json:"tools,omitempty"`
	Format    any            `json:"format,omitempty"`
	Think     any            `json:"think,omitempty"`
	KeepAlive any            `json:"keep_alive,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
	LogProbs    bool         `json:"logprobs,omitempty"`
	TopLogProbs int          `json:"top_logprobs,omitempty"`
}

// ChatResponse is one frame in the /api/chat NDJSON stream.
type ChatResponse struct {
	Model      string  `json:"model"`
	CreatedAt  string  `json:"created_at"`
	Message    Message `json:"message"`
	Done       bool    `json:"done"`
	DoneReason string  `json:"done_reason,omitempty"`
	GenerationStats
}

// ─── /api/embed ──────────────────────────────────────────────────────────────

// EmbedRequest is the body for POST /api/embed.
type EmbedRequest struct {
	Model     string         `json:"model"`
	Input     any            `json:"input"` // string | []string
	Truncate  *bool          `json:"truncate,omitempty"`
	Dimensions int           `json:"dimensions,omitempty"`
	KeepAlive string         `json:"keep_alive,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

// EmbedResponse is returned by POST /api/embed.
type EmbedResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float64 `json:"embeddings"`
	TotalDuration   int64       `json:"total_duration,omitempty"`
	LoadDuration    int64       `json:"load_duration,omitempty"`
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
}

// ─── /api/tags ───────────────────────────────────────────────────────────────

// ModelInfo is one entry in the /api/tags model list.
type ModelInfo struct {
	Name       string       `json:"name"`
	Model      string       `json:"model"`
	ModifiedAt string       `json:"modified_at"`
	Size       int64        `json:"size"`
	Digest     string       `json:"digest"`
	Details    ModelDetails `json:"details"`
}

// TagsResponse is returned by GET /api/tags.
type TagsResponse struct {
	Models []ModelInfo `json:"models"`
}

// ─── /api/ps ─────────────────────────────────────────────────────────────────

// RunningModel is one entry in the /api/ps running model list.
type RunningModel struct {
	Model         string       `json:"model"`
	Size          int64        `json:"size"`
	Digest        string       `json:"digest"`
	Details       ModelDetails `json:"details"`
	ExpiresAt     string       `json:"expires_at"`
	SizeVRAM      int64        `json:"size_vram"`
	ContextLength int          `json:"context_length"`
}

// PSResponse is returned by GET /api/ps.
type PSResponse struct {
	Models []RunningModel `json:"models"`
}

// ─── /api/show ───────────────────────────────────────────────────────────────

// ShowRequest is the body for POST /api/show.
type ShowRequest struct {
	Model   string `json:"model"`
	Verbose bool   `json:"verbose,omitempty"`
}

// ShowResponse is returned by POST /api/show.
type ShowResponse struct {
	Parameters   string         `json:"parameters,omitempty"`
	License      string         `json:"license,omitempty"`
	Capabilities []string       `json:"capabilities,omitempty"`
	ModifiedAt   string         `json:"modified_at,omitempty"`
	Template     string         `json:"template,omitempty"`
	Details      ModelDetails   `json:"details"`
	ModelInfo    map[string]any `json:"model_info,omitempty"`
}

// ─── /api/create ─────────────────────────────────────────────────────────────

// CreateRequest is the body for POST /api/create.
type CreateRequest struct {
	Model      string         `json:"model"`
	From       string         `json:"from,omitempty"`
	Template   string         `json:"template,omitempty"`
	License    any            `json:"license,omitempty"` // string | []string
	System     string         `json:"system,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
	Messages   []Message      `json:"messages,omitempty"`
	Quantize   string         `json:"quantize,omitempty"`
	Stream     *bool          `json:"stream,omitempty"`
}

// ─── /api/copy ───────────────────────────────────────────────────────────────

// CopyRequest is the body for POST /api/copy.
type CopyRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// ─── /api/pull ───────────────────────────────────────────────────────────────

// PullRequest is the body for POST /api/pull.
type PullRequest struct {
	Model    string `json:"model"`
	Insecure bool   `json:"insecure,omitempty"`
	Stream   *bool  `json:"stream,omitempty"`
}

// ─── /api/push ───────────────────────────────────────────────────────────────

// PushRequest is the body for POST /api/push.
type PushRequest struct {
	Model    string `json:"model"`
	Insecure bool   `json:"insecure,omitempty"`
	Stream   *bool  `json:"stream,omitempty"`
}

// ─── /api/delete ─────────────────────────────────────────────────────────────

// DeleteRequest is the body for DELETE /api/delete.
type DeleteRequest struct {
	Model string `json:"model"`
}

// ─── Streaming status frames (pull/push/create share this shape) ─────────────

// StatusFrame is one NDJSON frame emitted by pull/push/create endpoints.
type StatusFrame struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
	Error     string `json:"error,omitempty"`
}
