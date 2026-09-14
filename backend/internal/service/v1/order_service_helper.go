package v1service

import (
	"errors"
	"fmt"
	"goshop/internal/utils"
	"goshop/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InvalidateOrderCaches clears product, order, coupon, and user cart caches.
func (s *orderService) InvalidateOrderCaches(userUUID uuid.UUID) {
	if err := s.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	if err := s.cache.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}
	if err := s.cache.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}
	if userUUID != uuid.Nil {
		if err := s.cache.Clear(fmt.Sprintf("cart:user:%s", userUUID.String())); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
		}
	}
}

func (s *orderService) handleUserNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
}

func (s *orderService) handleOrderNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
}
