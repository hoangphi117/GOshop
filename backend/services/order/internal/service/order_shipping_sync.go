package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goshop/pkg/db"
	"goshop/pkg/logger"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/utils"
	"goshop/services/order/internal/db/sqlc"
	v1dto "goshop/services/order/internal/dto"

	"github.com/google/uuid"
)

func (s *orderShippingService) SyncOrderTracking(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.SyncTrackingResponseDTO{}, err
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return v1dto.SyncTrackingResponseDTO{}, s.handleOrderNotFoundError(err)
	}

	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.SyncTrackingResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.SyncTrackingResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			"Order does not have a shipping tracking code to synchronize",
		)
	}
	shippingCode := strings.TrimSpace(*order.OrderShippingCode)

	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = strings.TrimSpace(*order.OrderShippingProvider)
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Unsupported shipping provider", err)
	}

	trackingInfo, err := provider.GetTrackingInfo(ctx, shippingCode)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("order_number", order.OrderNumber).
			Str("shipping_code", shippingCode).
			Msg("Failed to fetch tracking info from shipping provider")
		return v1dto.SyncTrackingResponseDTO{}, utils.WrapError(
			utils.ErrCodeBadGateway,
			"Failed to query tracking information from GHN gateway",
			err,
		)
	}

	ghnStatus := strings.ToLower(strings.TrimSpace(trackingInfo.Status))

	isDeliveredNow := false
	if ghnStatus == "delivered" && order.OrderStatus != OrderStatusDelivered {
		err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
			tx, _ := db.GetTx(txCtx)
			_, err = s.orderRepo.MarkDeliveredByShippingCode(txCtx, tx, shippingCode)
			if err != nil {
				return err
			}

			if s.trackingRepo != nil {
				eventNote := fmt.Sprintf("GHN Sync: %s", trackingInfo.StatusText)
				_, _ = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
					EventOrderID: order.OrderID,
					EventStatus:  "delivered",
					EventNote:    &eventNote,
					EventSource:  "ghn_sync",
				})
			}
			return nil
		})
		if err == nil {
			isDeliveredNow = true
			if s.cache != nil {
				_ = s.cache.Clear(fmt.Sprintf("order:%s", orderUUID.String()))
				_ = s.cache.Clear("orders:*")
			}

			if s.rabbitmq != nil {
				evt := rabbitmq.NewOrderStatusChangedEvent(
					uuid.New().String(),
					orderUUID.String(),
					order.OrderNumber,
					userUUID.String(),
					order.OrderID,
					order.OrderUserID,
					order.OrderStatus,
					OrderStatusDelivered,
					shippingCode,
				)
				_ = s.rabbitmq.Publish(context.Background(), rabbitmq.QueueOrderStatusChanged, evt)
				_ = s.rabbitmq.PublishTopic(context.Background(), rabbitmq.RoutingKeyOrderStatusChanged, evt)
			}
		}
	} else {
		existingEvents, _ := s.trackingRepo.GetByOrderID(ctx, order.OrderID)
		hasStatusAlready := false
		for _, ev := range existingEvents {
			if strings.EqualFold(ev.EventStatus, ghnStatus) {
				hasStatusAlready = true
				break
			}
		}

		if !hasStatusAlready && ghnStatus != "" {
			eventNote := fmt.Sprintf("GHN tracking update: %s", trackingInfo.StatusText)
			_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
				EventOrderID: order.OrderID,
				EventStatus:  ghnStatus,
				EventNote:    &eventNote,
				EventSource:  "ghn_sync",
			})
		}
	}

	updatedEvents, _ := s.trackingRepo.GetByOrderID(ctx, order.OrderID)

	return v1dto.SyncTrackingResponseDTO{
		OrderUUID:        order.OrderUuid.String(),
		OrderNumber:      order.OrderNumber,
		ShippingCode:     shippingCode,
		ShippingProvider: strings.ToUpper(providerName),
		CurrentStatus:    ghnStatus,
		StatusText:       trackingInfo.StatusText,
		IsDelivered:      order.OrderStatus == OrderStatusDelivered || isDeliveredNow,
		LastSyncAt:       time.Now().UTC(),
		Timeline:         v1dto.MapToEventDTOs(updatedEvents),
		Message:          "Order tracking information synchronized successfully from GHN.",
	}, nil
}

func (s *orderShippingService) AdminSyncActiveShippingOrders(ctx context.Context, userUUID uuid.UUID) (v1dto.SyncActiveShippingStatsDTO, error) {
	user, err := s.getCustomer(ctx, userUUID)
	if err != nil {
		return v1dto.SyncActiveShippingStatsDTO{}, err
	}

	if user.UserLevel != RoleAdmin && user.UserLevel != RoleModerator {
		return v1dto.SyncActiveShippingStatsDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	return s.SyncActiveShippingOrders(ctx)
}

func (s *orderShippingService) SyncActiveShippingOrders(ctx context.Context) (v1dto.SyncActiveShippingStatsDTO, error) {
	const batchLimit int32 = 50
	orders, err := s.orderRepo.GetActiveShippedOrders(ctx, batchLimit)
	if err != nil {
		logger.Log.Error().Err(err).Msg("Failed to query active shipped orders for sync cronjob")
		return v1dto.SyncActiveShippingStatsDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to query active shipped orders", err)
	}

	stats := v1dto.SyncActiveShippingStatsDTO{
		ScannedCount: len(orders),
		SyncedAt:     time.Now().UTC(),
	}

	if len(orders) == 0 {
		logger.Log.Debug().Msg("No active shipped orders found for cronjob synchronization")
		return stats, nil
	}

	logger.Log.Info().
		Int("orders_to_scan", len(orders)).
		Msg("Starting background shipping sync cronjob batch")

	for _, order := range orders {
		select {
		case <-ctx.Done():
			logger.Log.Warn().Msg("Shipping cronjob synchronization aborted due to context cancellation")
			return stats, ctx.Err()
		default:
		}

		if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
			continue
		}
		shippingCode := strings.TrimSpace(*order.OrderShippingCode)

		providerName := "ghn"
		if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
			providerName = strings.TrimSpace(*order.OrderShippingProvider)
		}

		provider, err := s.shippingFactory.GetProvider(providerName)
		if err != nil {
			logger.Log.Warn().
				Str("provider", providerName).
				Str("order_number", order.OrderNumber).
				Msg("Shipping provider not supported in sync cronjob")
			stats.FailedCount++
			continue
		}

		trackingInfo, err := provider.GetTrackingInfo(ctx, shippingCode)
		if err != nil {
			logger.Log.Warn().
				Err(err).
				Str("order_number", order.OrderNumber).
				Str("shipping_code", shippingCode).
				Msg("Failed to query tracking info in sync cronjob")
			stats.FailedCount++
			time.Sleep(200 * time.Millisecond)
			continue
		}

		stats.SyncedCount++
		ghnStatus := strings.ToLower(strings.TrimSpace(trackingInfo.Status))

		if ghnStatus == "delivered" && order.OrderStatus != OrderStatusDelivered {
			err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
				tx, _ := db.GetTx(txCtx)
				_, err = s.orderRepo.MarkDeliveredByShippingCode(txCtx, tx, shippingCode)
				if err != nil {
					return err
				}

				if s.trackingRepo != nil {
					eventNote := fmt.Sprintf("Cronjob Sync: %s", trackingInfo.StatusText)
					_, err = s.trackingRepo.InsertEvent(txCtx, tx, sqlc.InsertTrackingEventParams{
						EventOrderID: order.OrderID,
						EventStatus:  "delivered",
						EventNote:    &eventNote,
						EventSource:  "ghn_cron",
					})
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				logger.Log.Error().Err(err).Str("order_number", order.OrderNumber).Msg("Failed to mark order delivered in sync cronjob")
				stats.FailedCount++
				continue
			}

			stats.DeliveredCount++
			if s.cache != nil {
				_ = s.cache.Clear(fmt.Sprintf("order:%s", order.OrderUuid.String()))
				_ = s.cache.Clear("orders:*")
			}

			if s.rabbitmq != nil {
				evt := rabbitmq.NewOrderStatusChangedEvent(
					uuid.New().String(),
					order.OrderUuid.String(),
					order.OrderNumber,
					"",
					order.OrderID,
					order.OrderUserID,
					order.OrderStatus,
					OrderStatusDelivered,
					shippingCode,
				)
				_ = s.rabbitmq.Publish(context.Background(), rabbitmq.QueueOrderStatusChanged, evt)
				_ = s.rabbitmq.PublishTopic(context.Background(), rabbitmq.RoutingKeyOrderStatusChanged, evt)
			}

			logger.Log.Info().
				Str("order_number", order.OrderNumber).
				Str("shipping_code", shippingCode).
				Msg("Order transitioned to DELIVERED via shipping sync cronjob")
		} else if ghnStatus != "" {
			existingEvents, err := s.trackingRepo.GetByOrderID(ctx, order.OrderID)
			if err == nil {
				hasStatusAlready := false
				for _, ev := range existingEvents {
					if strings.EqualFold(ev.EventStatus, ghnStatus) {
						hasStatusAlready = true
						break
					}
				}
				if !hasStatusAlready {
					eventNote := fmt.Sprintf("GHN update: %s", trackingInfo.StatusText)
					_, _ = s.trackingRepo.InsertEvent(ctx, nil, sqlc.InsertTrackingEventParams{
						EventOrderID: order.OrderID,
						EventStatus:  ghnStatus,
						EventNote:    &eventNote,
						EventSource:  "ghn_cron",
					})
				}
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	logger.Log.Info().
		Int("scanned", stats.ScannedCount).
		Int("synced", stats.SyncedCount).
		Int("delivered", stats.DeliveredCount).
		Int("failed", stats.FailedCount).
		Msg("Shipping sync cronjob batch completed successfully")

	return stats, nil
}
