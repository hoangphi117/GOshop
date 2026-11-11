package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSearchHistorySubject is returned when neither a cookie nor a user id is
// available; the handler turns it into a 401.
var ErrSearchHistorySubject = errors.New("search history: no subject")

// ErrEventRejected marks a validation failure the client caused (unknown event
// type, missing fields). Handlers map it to 400 instead of 500 so a bad client
// payload is not reported as a server fault.
type ErrEventRejected struct{ Reason string }

func (e *ErrEventRejected) Error() string { return "event rejected: " + e.Reason }

// EngagementEvent is one browser interaction, already attributed to a subject.
type EngagementEvent struct {
	EventID     uuid.UUID
	EventType   string
	AnonID      *uuid.UUID
	UserID      *int32
	SessionID   *uuid.UUID
	Query       *string
	ProductUUID *uuid.UUID
	Position    *int16
	ResultCount *int32
	Props       map[string]any
	OccurredAt  time.Time
}

// allowedEventTypes is the closed taxonomy. Anything else is a client bug and
// is rejected loudly rather than stored where no consumer can use it.
var allowedEventTypes = map[string]struct{}{
	"search":       {},
	"search_click": {},
	"view_item":    {},
	"add_to_cart":  {},
	"purchase":     {},
}

// IsAllowedEventType reports whether the taxonomy accepts the type.
func IsAllowedEventType(eventType string) bool {
	_, ok := allowedEventTypes[eventType]
	return ok
}

// MaxEventAge drops batches replayed from a long-suspended device: they would
// distort the popularity windows without adding signal.
const MaxEventAge = 7 * 24 * time.Hour

// EventRepository stores interaction telemetry and serves search history.
type EventRepository interface {
	InsertEvents(ctx context.Context, events []EngagementEvent) (int, error)
	ListSearchHistory(ctx context.Context, kind, subject string, limit int32) ([]SearchHistoryEntry, error)
	DismissSearch(ctx context.Context, kind, subject, query string) error
	DismissAllSearches(ctx context.Context, kind, subject string) error
}

// SearchHistoryEntry is one distinct keyword with its most recent timestamp.
type SearchHistoryEntry struct {
	Query     string    `json:"query"`
	LastSeen  time.Time `json:"last_seen"`
	SearchCnt int32     `json:"search_count"`
}

type sqlEventRepository struct {
	pool *pgxpool.Pool
}

func NewSqlEventRepository(pool *pgxpool.Pool) EventRepository {
	return &sqlEventRepository{pool: pool}
}

func (r *sqlEventRepository) InsertEvents(ctx context.Context, events []EngagementEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	batch := &pgx.Batch{}
	for _, e := range events {
		batch.Queue(`
			INSERT INTO user_events (
				event_id, event_type, anon_id, user_id, session_id, query,
				product_uuid, position, result_count, props, occurred_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (event_id) DO NOTHING`,
			e.EventID, e.EventType, e.AnonID, e.UserID, e.SessionID, e.Query,
			e.ProductUUID, e.Position, e.ResultCount, e.Props, e.OccurredAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	stored := 0
	for range events {
		tag, err := results.Exec()
		if err != nil {
			return stored, fmt.Errorf("insert engagement event: %w", err)
		}
		stored += int(tag.RowsAffected())
	}
	return stored, nil
}

// subjectColumn maps a subject kind to its column so the query stays
// parameterised while still using the per-subject indexes.
func subjectColumn(kind string) (string, error) {
	switch kind {
	case "anon":
		return "anon_id", nil
	case "user":
		return "user_id", nil
	default:
		return "", ErrSearchHistorySubject
	}
}

func (r *sqlEventRepository) ListSearchHistory(ctx context.Context, kind, subject string, limit int32) ([]SearchHistoryEntry, error) {
	col, err := subjectColumn(kind)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// DISTINCT ON keeps one row per keyword (the newest), then the outer sort
	// orders by recency so the dropdown renders newest-first.
	query := fmt.Sprintf(`
		SELECT query, last_seen, search_cnt FROM (
			SELECT DISTINCT ON (lower(e.query))
				e.query AS query,
				max(e.occurred_at) OVER (PARTITION BY lower(e.query)) AS last_seen,
				count(*) OVER (PARTITION BY lower(e.query))::int AS search_cnt,
				e.occurred_at
			FROM user_events e
			WHERE e.event_type = 'search'
			  AND e.%[1]s = $1
			  AND e.query IS NOT NULL
			  AND e.query <> ''
			  AND (e.result_count IS NULL OR e.result_count > 0)
			  AND e.occurred_at > now() - interval '90 days'
			  AND NOT EXISTS (
				  SELECT 1 FROM search_history_dismissals d
				  WHERE d.subject_kind = $2
				    AND d.subject_id = $3
				    AND lower(d.query) = lower(e.query)
				    AND d.dismissed_at >= e.occurred_at
			  )
			ORDER BY lower(e.query), e.occurred_at DESC
		) t
		ORDER BY last_seen DESC
		LIMIT $4`, col)

	rows, err := r.pool.Query(ctx, query, subject, kind, subject, limit)
	if err != nil {
		return nil, fmt.Errorf("list search history: %w", err)
	}
	defer rows.Close()

	out := make([]SearchHistoryEntry, 0, limit)
	for rows.Next() {
		var entry SearchHistoryEntry
		if err := rows.Scan(&entry.Query, &entry.LastSeen, &entry.SearchCnt); err != nil {
			return nil, fmt.Errorf("scan search history: %w", err)
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *sqlEventRepository) DismissSearch(ctx context.Context, kind, subject, query string) error {
	if _, err := subjectColumn(kind); err != nil {
		return err
	}
	if query == "" {
		return errors.New("search history: empty query")
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO search_history_dismissals (subject_kind, subject_id, query)
		VALUES ($1, $2, lower($3))
		ON CONFLICT (subject_kind, subject_id, query) DO UPDATE SET dismissed_at = now()`,
		kind, subject, query)
	if err != nil {
		return fmt.Errorf("dismiss search: %w", err)
	}
	return nil
}

// DismissAllSearches hides every keyword seen so far without touching the log.
func (r *sqlEventRepository) DismissAllSearches(ctx context.Context, kind, subject string) error {
	col, err := subjectColumn(kind)
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		INSERT INTO search_history_dismissals (subject_kind, subject_id, query)
		SELECT $1, $2, lower(e.query)
		FROM user_events e
		WHERE e.event_type = 'search' AND e.%s = $3 AND e.query IS NOT NULL AND e.query <> ''
		GROUP BY lower(e.query)
		ON CONFLICT (subject_kind, subject_id, query) DO UPDATE SET dismissed_at = now()`, col)

	if _, err := r.pool.Exec(ctx, query, kind, subject, subject); err != nil {
		return fmt.Errorf("dismiss all searches: %w", err)
	}
	return nil
}
