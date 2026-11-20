package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"goshop/shared/pkg/llm"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// dbSessionStore keeps the transcript in Postgres.
//
// It replaced a Redis key holding []llm.Message: that shape is role+content
// only, so every product card the model had surfaced was gone after a reload.
// One table keyed by (user_id, conversation_uuid) also survives a Redis flush
// and a pod restart, which the in-memory store never did.
type dbSessionStore struct {
	pool *pgxpool.Pool
	// maxLoad caps what Load hands back to the prompt. Without it a
	// month-long conversation would be read in full on every turn.
	maxLoad int
}

// NewDBSessionStore builds the store. maxMessages is the cap on replayed
// messages (one user + one assistant per turn).
func NewDBSessionStore(pool *pgxpool.Pool, maxMessages int) SessionStore {
	if maxMessages <= 0 {
		maxMessages = 12
	}
	return &dbSessionStore{pool: pool, maxLoad: maxMessages}
}

// Load returns the stored turns, oldest first. A user with no history is not an
// error. The user_id predicate is what stops a caller who guesses somebody
// else's conversation UUID from reading it back into their own prompt.
func (s *dbSessionStore) Load(ctx context.Context, userID int32, sessionID string) ([]llm.Message, error) {
	if s.pool == nil || userID <= 0 || sessionID == "" {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT role, content
		  FROM assistant_messages
		 WHERE user_id = $1 AND conversation_uuid = $2::uuid
		 ORDER BY message_id DESC
		 LIMIT $3`, userID, sessionID, s.maxLoad)
	if err != nil {
		return nil, fmt.Errorf("assistant load: %w", err)
	}
	defer rows.Close()

	msgs := make([]llm.Message, 0, s.maxLoad)
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, fmt.Errorf("assistant load scan: %w", err)
		}
		msgs = append(msgs, llm.Message{Role: llm.Role(role), Content: content})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("assistant load rows: %w", err)
	}
	reverseMessages(msgs)
	return msgs, nil
}

// Append writes one turn. A failed append is not fatal for the customer: they
// still see the answer, they just lose it on the next reload.
func (s *dbSessionStore) Append(ctx context.Context, userID int32, sessionID string, turns ...Turn) error {
	if s.pool == nil || userID <= 0 || sessionID == "" || len(turns) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, t := range turns {
		products := t.Products
		if products == nil {
			products = []Product{}
		}
		raw, err := json.Marshal(products)
		if err != nil {
			return fmt.Errorf("assistant append encode: %w", err)
		}
		batch.Queue(`
			INSERT INTO assistant_messages
			       (conversation_uuid, user_id, role, content, products, fallback_to_human)
			VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6)`,
			sessionID, userID, t.Role, t.Content, raw, t.FallbackToHuman)
	}

	br := s.pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()
	for range turns {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("assistant append: %w", err)
		}
	}
	return nil
}

// Latest returns the newest conversation of the user. The subquery picks the
// conversation_uuid of their last message rather than storing a "current
// conversation" pointer: one less row to keep in sync.
func (s *dbSessionStore) Latest(ctx context.Context, userID int32, limit int) (Conversation, error) {
	if s.pool == nil || userID <= 0 {
		return Conversation{}, nil
	}
	if limit <= 0 {
		limit = 40
	}
	rows, err := s.pool.Query(ctx, `
		WITH newest AS (
		    SELECT conversation_uuid
		      FROM assistant_messages
		     WHERE user_id = $1
		     ORDER BY message_id DESC
		     LIMIT 1
		)
		SELECT m.conversation_uuid::text, m.role, m.content, m.products,
		       m.fallback_to_human, m.created_at
		  FROM assistant_messages m
		  JOIN newest n ON n.conversation_uuid = m.conversation_uuid
		 WHERE m.user_id = $1
		 ORDER BY m.message_id DESC
		 LIMIT $2`, userID, limit)
	if err != nil {
		return Conversation{}, fmt.Errorf("assistant latest: %w", err)
	}
	defer rows.Close()

	var conv Conversation
	for rows.Next() {
		var (
			turn     Turn
			products []byte
		)
		if err := rows.Scan(&conv.SessionID, &turn.Role, &turn.Content, &products,
			&turn.FallbackToHuman, &turn.CreatedAt); err != nil {
			return Conversation{}, fmt.Errorf("assistant latest scan: %w", err)
		}
		if len(products) > 0 {
			if err := json.Unmarshal(products, &turn.Products); err != nil {
				return Conversation{}, fmt.Errorf("assistant latest decode: %w", err)
			}
		}
		conv.Messages = append(conv.Messages, turn)
	}
	if err := rows.Err(); err != nil {
		return Conversation{}, fmt.Errorf("assistant latest rows: %w", err)
	}
	reverseTurns(conv.Messages)
	return conv, nil
}

// RunRetention deletes messages older than retention every interval.
//
// A goroutine instead of a cron container: the DELETE is idempotent, so the two
// api replicas sweeping at the same time is harmless, and there is nothing to
// deploy or monitor separately.
func RunRetention(ctx context.Context, pool *pgxpool.Pool, retention, interval time.Duration) {
	if pool == nil || retention <= 0 {
		return
	}
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-retention)
			tag, err := pool.Exec(ctx, `DELETE FROM assistant_messages WHERE created_at < $1`, cutoff)
			if err != nil {
				log.Warn().Err(err).Msg("Assistant transcript retention sweep failed")
				continue
			}
			log.Info().
				Int64("deleted", tag.RowsAffected()).
				Time("cutoff", cutoff).
				Msg("Assistant transcript retention sweep done")
		}
	}
}

func reverseMessages(msgs []llm.Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}

func reverseTurns(turns []Turn) {
	for i, j := 0, len(turns)-1; i < j; i, j = i+1, j-1 {
		turns[i], turns[j] = turns[j], turns[i]
	}
}
