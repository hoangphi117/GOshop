package dto

// ChatRequest is the POST /api/v1/assistant/chat body.
//
// history is optional: the server replays its own Redis session when
// session_id is set. Sending both is allowed and the server de-duplicates by
// keeping the last maxHistoryMessages turns.
type ChatRequest struct {
	SessionID string          `json:"session_id" binding:"omitempty,uuid4"`
	Message   string          `json:"message"    binding:"required,min=1,max=2000"`
	History   []AssistantTurn `json:"history"    binding:"omitempty,max=20,dive"`
}

// AssistantTurn is one user/assistant exchange echoed by the client.
type AssistantTurn struct {
	Role    string `json:"role"    binding:"required,oneof=user assistant"`
	Content string `json:"content" binding:"required,max=2000"`
}

// ChatResponse is what the storefront renders. Answer is model prose; Products
// carries live price/stock so the UI never trusts numbers inside Answer.
type ChatResponse struct {
	SessionID       string    `json:"session_id"`
	Answer          string    `json:"answer"`
	Products        []Product `json:"products"`
	FallbackToHuman bool      `json:"fallback_to_human"`
	Model           string    `json:"model,omitempty"`
	Usage           *Usage    `json:"usage,omitempty"`
}

// Product is a catalog projection safe to hand to the browser.
type Product struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"`
	CategoryName string  `json:"category_name,omitempty"`
	ImageURL     string  `json:"image_url,omitempty"`
}

// Usage is the token accounting for one turn; omitempty on the parent keeps it
// off the wire when the provider reports nothing.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	CachedTokens     int `json:"cached_tokens,omitempty"`
	ToolCalls        int `json:"tool_calls,omitempty"`
}

// SessionResponse is returned by POST /api/v1/assistant/session.
type SessionResponse struct {
	SessionID string `json:"session_id"`
	ExpiresIn int    `json:"expires_in"`
}
