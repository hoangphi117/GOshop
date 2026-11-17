package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	analyticsdto "goshop/services/analytics/internal/dto"
	"goshop/services/analytics/internal/repository"
	"goshop/shared/pkg/utils"
)

// subject resolves the identity a request writes to. Signed-in users key by
// user id (durable, survives a cookie clear); guests key by the anon cookie.
func subject(anonID string, userID int32) (kind, id string, err error) {
	if userID > 0 {
		return "user", strconv.FormatInt(int64(userID), 10), nil
	}
	if anonID != "" {
		if _, perr := uuid.Parse(anonID); perr == nil {
			return "anon", anonID, nil
		}
	}
	return "", "", repository.ErrSearchHistorySubject
}

func (s *eventService) Ingest(ctx context.Context, anonID string, userID int32, inputs []analyticsdto.EngagementEventInput) (int, error) {
	if len(inputs) == 0 {
		return 0, nil
	}
	if len(inputs) > MaxEventsPerBatch {
		return 0, utils.NewError(utils.ErrCodeBadRequest, "too many events in one batch")
	}

	var anon *uuid.UUID
	if anonID != "" {
		if u, err := uuid.Parse(anonID); err == nil {
			anon = &u
		}
	}
	var uid *int32
	if userID > 0 {
		id := userID
		uid = &id
	}
	if anon == nil && uid == nil {
		return 0, repository.ErrSearchHistorySubject
	}

	now := time.Now().UTC()
	events := make([]repository.EngagementEvent, 0, len(inputs))
	for _, in := range inputs {
		e, err := buildEvent(in, anon, uid, now)
		if err != nil {
			return 0, err
		}
		events = append(events, e)
	}

	stored, err := s.repo.InsertEvents(ctx, events)
	if err != nil {
		return stored, err
	}
	s.publishTelemetry(ctx, events)
	return stored, nil
}

func buildEvent(in analyticsdto.EngagementEventInput, anon *uuid.UUID, uid *int32, now time.Time) (repository.EngagementEvent, error) {
	eventID, err := uuid.Parse(strings.TrimSpace(in.EventID))
	if err != nil {
		return repository.EngagementEvent{}, &repository.ErrEventRejected{Reason: "invalid event_id"}
	}
	if !repository.IsAllowedEventType(in.EventType) {
		return repository.EngagementEvent{}, &repository.ErrEventRejected{Reason: "unknown event_type: " + in.EventType}
	}

	occurred := now
	if in.OccurredAt != "" {
		parsed, perr := time.Parse(time.RFC3339, in.OccurredAt)
		if perr != nil {
			return repository.EngagementEvent{}, &repository.ErrEventRejected{Reason: "invalid occurred_at"}
		}
		// Reject implausible timestamps instead of storing garbage the
		// popularity windows would silently drop anyway.
		if parsed.After(now.Add(eventMaxSkew)) || parsed.Before(now.Add(-repository.MaxEventAge)) {
			return repository.EngagementEvent{}, &repository.ErrEventRejected{Reason: "occurred_at out of range"}
		}
		occurred = parsed.UTC()
	}

	e := repository.EngagementEvent{
		EventID:    eventID,
		EventType:  in.EventType,
		AnonID:     anon,
		UserID:     uid,
		Position:   in.Position,
		Props:      in.Props,
		OccurredAt: occurred,
	}
	if e.Props == nil {
		e.Props = map[string]any{}
	}

	if in.SessionID != "" {
		if u, perr := uuid.Parse(in.SessionID); perr == nil {
			e.SessionID = &u
		}
	}
	if in.ProductUUID != "" {
		u, perr := uuid.Parse(in.ProductUUID)
		if perr != nil {
			return repository.EngagementEvent{}, &repository.ErrEventRejected{Reason: "invalid product_uuid"}
		}
		e.ProductUUID = &u
	}
	if q := normalizeQuery(in.Query); q != "" {
		e.Query = &q
	}
	// A search with no keyword is not a search. Clicks need a target.
	switch in.EventType {
	case "search":
		if e.Query == nil {
			return repository.EngagementEvent{}, &repository.ErrEventRejected{Reason: "search requires query"}
		}
	case "view_item", "add_to_cart", "purchase":
		if e.ProductUUID == nil {
			return repository.EngagementEvent{}, &repository.ErrEventRejected{Reason: in.EventType + " requires product_uuid"}
		}
	}
	e.ResultCount = in.ResultCount
	return e, nil
}

// normalizeQuery collapses whitespace and drops control characters so
// "laptop " and " laptop" are one history entry.
func normalizeQuery(raw string) string {
	q := strings.Join(strings.Fields(raw), " ")
	if len(q) > 200 {
		q = q[:200]
	}
	return q
}

func (s *eventService) SearchHistory(ctx context.Context, anonID string, userID int32, limit int32) ([]repository.SearchHistoryEntry, error) {
	kind, id, err := subject(anonID, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListSearchHistory(ctx, kind, id, limit)
}

func (s *eventService) DismissSearch(ctx context.Context, anonID string, userID int32, query string) error {
	kind, id, err := subject(anonID, userID)
	if err != nil {
		return err
	}
	return s.repo.DismissSearch(ctx, kind, id, normalizeQuery(query))
}

func (s *eventService) DismissAllSearches(ctx context.Context, anonID string, userID int32) error {
	kind, id, err := subject(anonID, userID)
	if err != nil {
		return err
	}
	return s.repo.DismissAllSearches(ctx, kind, id)
}
