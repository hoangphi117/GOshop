package service

import (
	"context"
	"time"

	"goshop/shared/pkg/llm"
)

// Product is the catalog projection the assistant is allowed to talk about.
type Product struct {
	UUID         string  `json:"product_uuid"`
	Name         string  `json:"product_name"`
	Slug         string  `json:"product_slug"`
	Price        float64 `json:"product_price"`
	Stock        int32   `json:"product_stock"`
	CategoryName string  `json:"category_name,omitempty"`
	ImageURL     string  `json:"product_image_url,omitempty"`
}

// FAQ is one knowledge-base entry.
type FAQ struct {
	UUID     string `json:"faq_uuid"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// ProductSearcher is the free-text product port (catalog HTTP /products/search).
//
// pom.xml-grade laziness: this goes over the catalog HTTP route instead of a
// new gRPC RPC because catalog.CatalogClient has no search call, and adding one
// means touching the shared proto + three generated files for one query.
type ProductSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]Product, error)
}

// ProductReader is the exact-lookup port (catalog gRPC GetProduct).
type ProductReader interface {
	Get(ctx context.Context, uuid string) (Product, error)
}

// FAQSearcher is the FAQ port (Meilisearch index faqs).
type FAQSearcher interface {
	Search(ctx context.Context, query string, limit int) ([]FAQ, error)
}

// LLM is the model port. Kept as an alias of llm.Client so the service and the
// shared package cannot drift apart.
type LLM = llm.Client

// Limiter is the per-caller daily quota.
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// Turn is one stored exchange. It carries the product cards so a reload can
// redraw the same answer, which a plain role+content message cannot.
type Turn struct {
	Role            string
	Content         string
	Products        []Product
	FallbackToHuman bool
	CreatedAt       time.Time
}

// Conversation is the transcript the widget redraws after a reload.
type Conversation struct {
	SessionID string
	Messages  []Turn
}

// SessionStore is the transcript port. Every method is scoped by userID: the
// session id is client-supplied, so ownership has to come from the token, not
// from the id alone.
type SessionStore interface {
	Load(ctx context.Context, userID int32, sessionID string) ([]llm.Message, error)
	Append(ctx context.Context, userID int32, sessionID string, turns ...Turn) error
	// Latest returns the newest conversation of the user, at most limit turns.
	Latest(ctx context.Context, userID int32, limit int) (Conversation, error)
}

// Answer is what the handler returns to the storefront.
type Answer struct {
	Answer           string
	Products         []Product
	FallbackToHuman  bool
	Model            string
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	ToolCalls        int
}

// Tool is re-exported so callers do not need to import llm for the prompt.
type Tool = llm.Tool

// noopLimiter and noopSessionStore keep the service usable when Redis is not
// wired (unit tests, degraded boot). Deliberately not exported.
type noopLimiter struct{}

func (noopLimiter) Allow(context.Context, string) (bool, error) { return true, nil }

type noopSessionStore struct{}

func (noopSessionStore) Load(context.Context, int32, string) ([]llm.Message, error) {
	return nil, nil
}

func (noopSessionStore) Append(context.Context, int32, string, ...Turn) error { return nil }

func (noopSessionStore) Latest(context.Context, int32, int) (Conversation, error) {
	return Conversation{}, nil
}
