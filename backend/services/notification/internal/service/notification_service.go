package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"goshop/pkg/discord"
	"goshop/pkg/logger"
	"goshop/pkg/mail"
	"goshop/pkg/utils"
	"goshop/pkg/ws"
	identitycontract "goshop/services/identity/contract"
	"goshop/services/notification/internal/db/sqlc"
	"goshop/services/notification/internal/dto"
	"goshop/services/notification/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type notificationService struct {
	notificationRepo repository.NotificationRepository
	identityClient   identitycontract.IdentityClient
	mailService      mail.EmailProviderService
	discordNotifier  discord.Notifier
	redisBridge      ws.RedisBridge
}

func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	identityClient identitycontract.IdentityClient,
	mailService mail.EmailProviderService,
	discordNotifier discord.Notifier,
	redisBridge ws.RedisBridge,
) NotificationService {

	return &notificationService{
		notificationRepo: notificationRepo,
		identityClient:   identityClient,
		mailService:      mailService,
		discordNotifier:  discordNotifier,
		redisBridge:      redisBridge,
	}
}

func (s *notificationService) Send(ctx context.Context, input dto.SendNotificationInput) error {
	dataBytes, _ := s.marshalData(input.Data)

	params := sqlc.CreateNotificationParams{
		NotificationUserID: input.UserID,
		NotificationType:   input.Type,
		NotificationTitle:  input.Title,
		NotificationBody:   input.Body,
		NotificationData:   dataBytes,
	}

	created, err := s.notificationRepo.Create(ctx, params)
	if err != nil {
		logger.Log.Error().Err(err).
			Int32("user_id", input.UserID).
			Str("type", input.Type).
			Msg("Failed to persist in-app notification")
		return err
	}

	s.broadcastToUser(input.UserID, created)
	return nil
}

func (s *notificationService) CreateNotification(ctx context.Context, input dto.CreateNotificationInput) (dto.NotificationDTO, error) {
	var userID int32
	if s.identityClient != nil {
		user, err := s.identityClient.GetUserByUUID(ctx, input.UserUUID)
		if err != nil {
			return dto.NotificationDTO{}, utils.NewError(utils.ErrCodeNotFound, "Recipient user not found")
		}
		userID = user.UserID
	}

	dataBytes, err := s.marshalData(input.Data)
	if err != nil {
		return dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to marshal notification data payload")
	}

	params := sqlc.CreateNotificationParams{
		NotificationUserID: userID,
		NotificationType:   input.Type,
		NotificationTitle:  input.Title,
		NotificationBody:   input.Body,
		NotificationData:   dataBytes,
	}

	created, err := s.notificationRepo.Create(ctx, params)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to create notification")
		return dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to create notification")
	}

	s.broadcastToUser(userID, created)

	return dto.MapToNotificationDTO(created), nil
}

func (s *notificationService) BroadcastNotification(ctx context.Context, input dto.BroadcastNotificationInput) error {
	s.broadcastToAll(input)
	return nil
}

func (s *notificationService) GetMyNotifications(ctx context.Context, userUUID uuid.UUID, query dto.GetNotificationsQuery) ([]dto.NotificationDTO, int32, error) {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return nil, 0, err
	}

	limit, offset := s.normalizePagination(query)

	notifications, err := s.notificationRepo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", userID).Msg("Failed to get notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve notifications")
	}

	total, err := s.notificationRepo.CountByUserID(ctx, userID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", userID).Msg("Failed to count notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to count notifications")
	}

	dtos := make([]dto.NotificationDTO, 0, len(notifications))
	for _, n := range notifications {
		dtos = append(dtos, dto.MapToNotificationDTO(n))
	}

	return dtos, int32(total), nil
}

func (s *notificationService) GetUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (dto.UnreadCountDTO, error) {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return dto.UnreadCountDTO{}, err
	}

	count, err := s.notificationRepo.CountUnreadByUserID(ctx, userID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", userID).Msg("Failed to count unread notifications")
		return dto.UnreadCountDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve unread count")
	}

	return dto.UnreadCountDTO{UnreadCount: count}, nil
}

func (s *notificationService) MarkNotificationAsRead(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) (dto.NotificationDTO, error) {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return dto.NotificationDTO{}, err
	}

	updated, err := s.notificationRepo.MarkAsRead(ctx, notificationUUID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return dto.NotificationDTO{}, utils.NewError(utils.ErrCodeNotFound, "Notification not found")
		}
		logger.Log.Error().Err(err).
			Str("notification_uuid", notificationUUID.String()).
			Int32("user_id", userID).
			Msg("Failed to mark notification as read")
		return dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to mark notification as read")
	}

	return dto.MapToNotificationDTO(updated), nil
}

func (s *notificationService) MarkAllNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return err
	}

	err = s.notificationRepo.MarkAllAsRead(ctx, userID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", userID).Msg("Failed to mark all notifications as read")
		return utils.NewError(utils.ErrCodeInternal, "Failed to mark all notifications as read")
	}

	return nil
}

func (s *notificationService) DeleteNotification(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) error {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return err
	}

	err = s.notificationRepo.Delete(ctx, notificationUUID, userID)
	if err != nil {
		logger.Log.Error().Err(err).
			Str("notification_uuid", notificationUUID.String()).
			Int32("user_id", userID).
			Msg("Failed to delete notification")
		return utils.NewError(utils.ErrCodeInternal, "Failed to delete notification")
	}

	return nil
}

func (s *notificationService) NotifyAdmins(ctx context.Context, input dto.AdminNotifyInput) error {
	dataBytes, err := s.marshalData(input.Data)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to marshal notification data payload")
	}

	var adminIDs []int32
	if s.identityClient != nil {
		ids, err := s.identityClient.GetActiveAdminUserIDs(ctx)
		if err != nil {
			logger.Log.Error().Err(err).Str("type", input.Type).Msg("Failed to resolve admin recipients")
			return utils.NewError(utils.ErrCodeInternal, "Failed to resolve admin recipients")
		}
		adminIDs = ids
	}

	adminType := input.Type
	if !strings.HasPrefix(adminType, "admin_") {
		adminType = "admin_" + adminType
	}

	for _, adminID := range adminIDs {
		params := sqlc.CreateNotificationParams{
			NotificationUserID: adminID,
			NotificationType:   adminType,
			NotificationTitle:  input.Title,
			NotificationBody:   input.Body,
			NotificationData:   dataBytes,
		}
		if _, err := s.notificationRepo.Create(ctx, params); err != nil {
			logger.Log.Error().Err(err).Int32("admin_id", adminID).Str("type", adminType).Msg("Failed to create admin notification row")
			return utils.NewError(utils.ErrCodeInternal, "Failed to create admin notification")
		}
	}

	s.broadcastToAdmins(dto.NotificationDTO{
		UUID:      uuid.New(),
		Type:      adminType,
		Title:     input.Title,
		Body:      input.Body,
		Data:      input.Data,
		CreatedAt: time.Now().UTC(),
	})

	return nil
}

func (s *notificationService) GetAdminNotifications(ctx context.Context, userUUID uuid.UUID, query dto.AdminGetNotificationsQuery) ([]dto.NotificationDTO, int32, error) {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return nil, 0, err
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit

	notifications, err := s.notificationRepo.GetAdminNotifications(ctx, userID, query.Type, query.IsRead, limit, offset)
	if err != nil {
		logger.Log.Error().Err(err).Int32("admin_id", userID).Msg("Failed to fetch admin notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve admin notifications")
	}

	total, err := s.notificationRepo.CountAdminNotifications(ctx, userID, query.Type, query.IsRead)
	if err != nil {
		logger.Log.Error().Err(err).Int32("admin_id", userID).Msg("Failed to count admin notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to count admin notifications")
	}

	dtos := make([]dto.NotificationDTO, 0, len(notifications))
	for _, n := range notifications {
		dtos = append(dtos, dto.MapToNotificationDTO(n))
	}

	return dtos, int32(total), nil
}

func (s *notificationService) GetAdminUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (dto.UnreadCountDTO, error) {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return dto.UnreadCountDTO{}, err
	}

	count, err := s.notificationRepo.CountAdminUnreadNotifications(ctx, userID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("admin_id", userID).Msg("Failed to count admin unread notifications")
		return dto.UnreadCountDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve admin unread count")
	}

	return dto.UnreadCountDTO{UnreadCount: count}, nil
}

func (s *notificationService) MarkAllAdminNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error {
	userID, err := s.resolveUserID(ctx, userUUID)
	if err != nil {
		return err
	}

	err = s.notificationRepo.MarkAllAdminAsRead(ctx, userID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("admin_id", userID).Msg("Failed to mark all admin notifications as read")
		return utils.NewError(utils.ErrCodeInternal, "Failed to mark all admin notifications as read")
	}

	return nil
}

func (s *notificationService) PurgeStaleNotifications(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	count, err := s.notificationRepo.DeleteOldReadNotifications(ctx, cutoff)
	if err != nil {
		logger.Log.Error().Err(err).Int("retention_days", retentionDays).Msg("Failed to purge stale notifications")
		return 0, utils.NewError(utils.ErrCodeInternal, "Failed to purge stale notifications")
	}
	return count, nil
}

// ── Private Helpers ────────────────────────────────────

func (s *notificationService) resolveUserID(ctx context.Context, userUUID uuid.UUID) (int32, error) {
	if s.identityClient != nil {
		user, err := s.identityClient.GetUserByUUID(ctx, userUUID)
		if err != nil {
			return 0, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return user.UserID, nil
	}
	return 0, utils.NewError(utils.ErrCodeInternal, "Identity service not configured")
}

func (s *notificationService) marshalData(data any) ([]byte, error) {
	if data == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(data)
}

func (s *notificationService) broadcastToUser(userID int32, created sqlc.Notification) {
	wsMsg := ws.WSMessage{
		Type: ws.EventTypeNotification,
		Data: dto.MapToNotificationDTO(created),
	}
	s.publishLiveUpdate(&userID, false, wsMsg)
}

func (s *notificationService) broadcastToAdmins(dtoData dto.NotificationDTO) {
	wsMsg := ws.WSMessage{
		Type: ws.EventTypeNotification,
		Data: dtoData,
	}
	s.publishLiveUpdate(nil, true, wsMsg)
}

func (s *notificationService) broadcastToAll(input dto.BroadcastNotificationInput) {
	wsMsg := ws.WSMessage{
		Type: ws.EventTypeNotification,
		Data: map[string]any{
			"type":       input.Type,
			"title":      input.Title,
			"body":       input.Body,
			"data":       input.Data,
			"is_read":    false,
			"created_at": time.Now().UTC(),
		},
	}
	s.publishLiveUpdate(nil, false, wsMsg)
}

// publishLiveUpdate fans a notification out over the WebSocket bridge. Delivery
// is best-effort (the durable side effect is the notification row), but failures
// are logged, never silently discarded.
func (s *notificationService) publishLiveUpdate(userID *int32, isAdmin bool, wsMsg ws.WSMessage) {
	if s.redisBridge == nil {
		return
	}
	if err := s.redisBridge.Publish(context.Background(), ws.RedisNotificationEnvelope{
		UserID:  userID,
		IsAdmin: isAdmin,
		Message: wsMsg,
	}); err != nil {
		logger.Log.Warn().Err(err).Bool("is_admin", isAdmin).Msg("Failed to publish live notification over WebSocket bridge")
	}
}

func (s *notificationService) normalizePagination(query dto.GetNotificationsQuery) (limit, offset int32) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit = query.Limit
	if limit < 1 {
		limit = 10
	}
	offset = (page - 1) * limit
	return limit, offset
}
