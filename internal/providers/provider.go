// Package providers defines the LLM provider interface and shared types.
package providers

import "context"

// Role identifies the sender of a chat message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// ChatMessage is a single message in a conversation.
type ChatMessage struct {
	Role       Role   `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Tool describes a tool/function the LLM can call.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall is a request from the LLM to invoke a tool.
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Response is the LLM's reply to a chat request.
type Response struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Model     string     `json:"model"`
	Usage     Usage      `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent is a chunk of a streaming response.
type StreamEvent struct {
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Done     bool      `json:"done"`
	Err      error     `json:"-"`
}

// Provider is the interface all LLM backends implement.
type Provider interface {
	Chat(ctx context.Context, msgs []ChatMessage, tools []Tool) (*Response, error)
	StreamChat(ctx context.Context, msgs []ChatMessage, tools []Tool) (<-chan StreamEvent, error)
	ModelID() string
	Name() string
}
