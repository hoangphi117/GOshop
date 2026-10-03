package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type sqlSessionRepository struct {
	q sqlc.Querier
}

func NewSqlSessionRepository(q sqlc.Querier) SessionRepository {
	return &sqlSessionRepository{q: q}
}

func (r *sqlSessionRepository) Create(ctx context.Context, params sqlc.CreateSessionParams) (sqlc.UserSession, error) {
	return r.q.CreateSession(ctx, params)
}

func (r *sqlSessionRepository) ListByUserID(ctx context.Context, userID int32) ([]sqlc.UserSession, error) {
	return r.q.ListSessionsByUserID(ctx, userID)
}

func (r *sqlSessionRepository) GetByUUID(ctx context.Context, sessionUUID uuid.UUID) (sqlc.UserSession, error) {
	return r.q.GetSessionByUUID(ctx, sessionUUID)
}

func (r *sqlSessionRepository) GetByFamilyID(ctx context.Context, familyID string) (sqlc.UserSession, error) {
	return r.q.GetSessionByFamilyID(ctx, familyID)
}

func (r *sqlSessionRepository) DeleteByUUID(ctx context.Context, sessionUUID uuid.UUID, userID int32) error {
	return r.q.DeleteSessionByUUID(ctx, sqlc.DeleteSessionByUUIDParams{
		SessionUuid: sessionUUID,
		UserID:      userID,
	})
}

func (r *sqlSessionRepository) DeleteAllOther(ctx context.Context, userID int32, currentFamilyID string) error {
	return r.q.DeleteAllOtherSessions(ctx, sqlc.DeleteAllOtherSessionsParams{
		UserID:   userID,
		FamilyID: currentFamilyID,
	})
}

func (r *sqlSessionRepository) DeleteByFamilyID(ctx context.Context, familyID string) error {
	return r.q.DeleteSessionByFamilyID(ctx, familyID)
}

func (r *sqlSessionRepository) UpdateActivity(ctx context.Context, familyID string) error {
	return r.q.UpdateSessionActivity(ctx, familyID)
}
