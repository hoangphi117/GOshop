package v1service

import (
	"context"
	"database/sql"
	"errors"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SessionService interface {
	ListUserSessions(ctx context.Context, userUUID uuid.UUID, currentRefreshToken string) ([]v1dto.SessionDTO, error)
	RevokeSession(ctx context.Context, userUUID uuid.UUID, sessionUUID uuid.UUID) error
	RevokeAllOtherSessions(ctx context.Context, userUUID uuid.UUID, currentRefreshToken string) error
	RecordSession(ctx context.Context, userID int32, familyID, ip, userAgent string) error
}

type sessionService struct {
	sessionRepo  repository.SessionRepository
	userRepo     repository.UserRepository
	tokenService auth.TokenService
}

func NewSessionService(
	sessionRepo repository.SessionRepository,
	userRepo repository.UserRepository,
	tokenService auth.TokenService,
) SessionService {
	return &sessionService{
		sessionRepo:  sessionRepo,
		userRepo:     userRepo,
		tokenService: tokenService,
	}
}

func (s *sessionService) RecordSession(ctx context.Context, userID int32, familyID, ip, userAgent string) error {
	deviceName := utils.ParseDeviceName(userAgent)

	var ipPtr *string
	if ip != "" {
		ipPtr = &ip
	}

	var uaPtr *string
	if userAgent != "" {
		uaPtr = &userAgent
	}

	var devicePtr *string
	if deviceName != "" {
		devicePtr = &deviceName
	}

	_, err := s.sessionRepo.Create(ctx, sqlc.CreateSessionParams{
		UserID:     userID,
		FamilyID:   familyID,
		IpAddress:  ipPtr,
		UserAgent:  uaPtr,
		DeviceName: devicePtr,
	})
	return err
}

func (s *sessionService) ListUserSessions(ctx context.Context, userUUID uuid.UUID, currentRefreshToken string) ([]v1dto.SessionDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
	}

	currentFamilyID := ""
	if currentRefreshToken != "" && s.tokenService != nil {
		currentFamilyID, _ = s.tokenService.GetTokenFamilyID(currentRefreshToken)
	}

	sessions, err := s.sessionRepo.ListByUserID(ctx, user.UserID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to list user sessions", err)
	}

	dtos := make([]v1dto.SessionDTO, 0, len(sessions))
	for _, sess := range sessions {
		ip := ""
		if sess.IpAddress != nil {
			ip = *sess.IpAddress
		}
		ua := ""
		if sess.UserAgent != nil {
			ua = *sess.UserAgent
		}
		dev := ""
		if sess.DeviceName != nil {
			dev = *sess.DeviceName
		}

		dtos = append(dtos, v1dto.SessionDTO{
			UUID:         sess.SessionUuid.String(),
			IPAddress:    ip,
			DeviceName:   dev,
			UserAgent:    ua,
			LastActiveAt: sess.LastActiveAt,
			CreatedAt:    sess.CreatedAt,
			IsCurrent:    currentFamilyID != "" && sess.FamilyID == currentFamilyID,
		})
	}

	return dtos, nil
}

func (s *sessionService) RevokeSession(ctx context.Context, userUUID uuid.UUID, sessionUUID uuid.UUID) error {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	sess, err := s.sessionRepo.GetByUUID(ctx, sessionUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Session not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to get session", err)
	}

	if sess.UserID != user.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "Unauthorized to terminate this session")
	}

	// Revoke in Redis
	if s.tokenService != nil && sess.FamilyID != "" {
		_ = s.tokenService.RevokeTokenFamily(sess.FamilyID)
	}

	// Delete from DB
	return s.sessionRepo.DeleteByUUID(ctx, sessionUUID, user.UserID)
}

func (s *sessionService) RevokeAllOtherSessions(ctx context.Context, userUUID uuid.UUID, currentRefreshToken string) error {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	currentFamilyID := ""
	if currentRefreshToken != "" && s.tokenService != nil {
		currentFamilyID, _ = s.tokenService.GetTokenFamilyID(currentRefreshToken)
	}

	if currentFamilyID == "" {
		return utils.NewError(utils.ErrCodeBadRequest, "Cannot identify current session")
	}

	sessions, err := s.sessionRepo.ListByUserID(ctx, user.UserID)
	if err == nil {
		for _, sess := range sessions {
			if sess.FamilyID != currentFamilyID && s.tokenService != nil {
				_ = s.tokenService.RevokeTokenFamily(sess.FamilyID)
			}
		}
	}

	return s.sessionRepo.DeleteAllOther(ctx, user.UserID, currentFamilyID)
}
