package providers

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrNotSupported is returned by adapters for operations they cannot implement
// (e.g., ListModels against Ollama).
var ErrNotSupported = errors.New("operation not supported by provider")

// Role values used across the interface.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Content part types.
const (
	PartText  = "text"
	PartImage = "image"
	PartFile  = "file"
)

// Chunk types emitted on the streaming channel.
const (
	ChunkTextDelta     = "text_delta"
	ChunkToolCallStart = "tool_call_start"
	ChunkToolCallDelta = "tool_call_delta"
	ChunkThinkingDelta = "thinking_delta"
	ChunkMessageStop   = "message_stop"
	ChunkUsage         = "usage"
	ChunkError         = "error"
)

// Stop reasons.
const (
	StopEndTurn      = "end_turn"
	StopMaxTokens    = "max_tokens"
	StopToolUse      = "tool_use"
	StopStopSequence = "stop_sequence"
	StopError        = "error"
)

type Message struct {
	Role       string        `json:"role"`
	Content    []ContentPart `json:"content,omitempty"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Data     []byte `json:"data,omitempty"` // JSON-encoded as base64
}

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"input_schema"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

type Chunk struct {
	Type       string    `json:"type"`
	Text       string    `json:"text,omitempty"`
	ToolCall   *ToolCall `json:"tool_call,omitempty"`
	Usage      *Usage    `json:"usage,omitempty"`
	StopReason string    `json:"stop_reason,omitempty"` // set on message_stop
	Err        error     `json:"-"`
}

type Request struct {
	Model       string
	Messages    []Message
	System      string
	Tools       []ToolDef
	Temperature *float64
	MaxTokens   *int
	Stream      bool
}

type Response struct {
	Messages   []Message
	Usage      Usage
	StopReason string
}

type ModelInfo struct {
	ID              string
	DisplayName     string
	ContextTokens   int
	InputPricePerM  *float64
	OutputPricePerM *float64
}

// Provider is the single abstraction every adapter implements.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (<-chan Chunk, error)
	ListModels(ctx context.Context) ([]ModelInfo, error)
}
