package v1service

import (
	"errors"
	"fmt"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InvalidateOrderCaches clears product, order, coupon, and user cart caches.
func InvalidateOrderCachesWithCache(c cache.RedisCacheService, userUUID uuid.UUID) {
	if c == nil {
		return
	}
	if err := c.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	if err := c.Clear("orders:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear order cache")
	}
	if err := c.Clear("coupons:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear coupon cache")
	}
	if userUUID != uuid.Nil {
		if err := c.Clear(fmt.Sprintf("cart:user:%s", userUUID.String())); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear cart cache")
		}
	}
}

func (s *orderService) InvalidateOrderCaches(userUUID uuid.UUID) {
	if s.checkoutService != nil {
		InvalidateOrderCachesWithCache(s.cache, userUUID)
	}
}

func handleUserNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
}

func handleOrderNotFoundError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}
	return utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
}

func (s *orderService) handleUserNotFoundError(err error) error {
	return handleUserNotFoundError(err)
}

func (s *orderService) handleOrderNotFoundError(err error) error {
	return handleOrderNotFoundError(err)
}
