// Package llm is the outbound port to a chat-completions provider.
//
// Only OpenAI-compatible providers are supported: River (https://api.river.ai/v1),
// OpenAI, Groq, DeepInfra, OpenRouter, Ollama. The wire format is documented at
// https://docs.river.ai/guides/inference/ and is the same subset OpenAI ships:
// POST {base}/chat/completions with tools + tool_calls.
//
// Native Anthropic (/v1/messages) and Gemini (/v1beta) use different tool-call
// shapes and need a separate implementation of Client. The assistant service
// depends on this interface, so adding one later is a new file, not a rewrite.
//
// ponytail: one adapter until a second native provider is actually required.
package llm

import (
	"context"
	"encoding/json"
)

// Role is the OpenAI chat role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is one turn of the conversation.
type Message struct {
	Role       Role
	Content    string
	ToolCallID string     // set when Role == RoleTool
	ToolCalls  []ToolCall // set when Role == RoleAssistant and the model asked for tools
}

// Tool is a function the model may call. Parameters is a JSON Schema object.
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

// ToolCall is one function call requested by the model.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON, validated by the caller before use
}

// Usage mirrors the provider token counters. CachedPromptTokens is the subset
// of PromptTokens billed at the cached rate; it is 0 when the provider omits it.
type Usage struct {
	PromptTokens       int
	CompletionTokens   int
	CachedPromptTokens int
}

// Reply is the model's answer for a single round trip.
type Reply struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        Usage
}

// Client is the LLM port.
//
// Implementations must be safe for concurrent use, must honour ctx for the
// whole request, and must not log the API key or the Authorization header.
type Client interface {
	Complete(ctx context.Context, msgs []Message, tools []Tool) (Reply, error)
}

// StreamingClient is the optional streaming half of Client. The assistant
// service type-asserts for it so a non-streaming provider (or a test fake)
// still works: it just serves the whole answer at once.
type StreamingClient interface {
	Client
	// CompleteStream emits text deltas through emit as they arrive and returns
	// the assembled reply. An error from emit aborts immediately.
	CompleteStream(ctx context.Context, msgs []Message, tools []Tool, emit func(Chunk) error) (Reply, error)
}
