package dto

// EngagementEventInput is one client-reported interaction. Identity is never
// taken from the body: the handler fills it from the session cookie / JWT so a
// caller cannot write events for someone else.
type EngagementEventInput struct {
	EventID     string         `json:"event_id" binding:"required"`
	EventType   string         `json:"event_type" binding:"required"`
	OccurredAt  string         `json:"occurred_at"`
	Query       string         `json:"query,omitempty"`
	ProductUUID string         `json:"product_uuid,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Position    *int16         `json:"position,omitempty"`
	ResultCount *int32         `json:"result_count,omitempty"`
	Props       map[string]any `json:"props,omitempty"`
}

// EngagementBatch is the request body of POST /api/v1/events.
type EngagementBatch struct {
	Events []EngagementEventInput `json:"events" binding:"required,dive"`
}

// SearchHistoryQuery is the query string of GET /api/v1/search/history.
type SearchHistoryQuery struct {
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=50"`
}
