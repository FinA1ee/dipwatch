// Package llm is a small, provider-neutral chat interface with tool calling.
// The agent depends only on Model, so tests can script a fake model and the
// provider (DeepSeek, Qwen, Kimi, OpenAI) is a configuration choice.
package llm

import "context"

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one entry in a conversation.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall // assistant messages only
	ToolCallID string     // tool messages only: which call this answers
}

// Tool describes a function the model may call. Parameters is a JSON Schema object.
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// ToolCall is the model asking to run a tool. Arguments is a JSON object as text.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// Turn is one model reply plus what it cost.
type Turn struct {
	Content   string
	ToolCalls []ToolCall
	TokensIn  int64
	TokensOut int64
}

type Model interface {
	// Name is the model id, recorded with every run.
	Name() string
	Chat(ctx context.Context, messages []Message, tools []Tool) (Turn, error)
	// CostMicros converts token counts to millionths of a yuan.
	CostMicros(tokensIn, tokensOut int64) int64
}
