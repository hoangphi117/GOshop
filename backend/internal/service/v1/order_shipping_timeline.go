package v1service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/shipping"

	"github.com/google/uuid"
)

// GetOrderTimeline retrieves the full tracking event history or synthesizes a baseline timeline.
func (s *orderShippingService) GetOrderTimeline(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		return v1dto.OrderTimelineResponseDTO{}, s.handleUserNotFoundError(err)
	}

	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return v1dto.OrderTimelineResponseDTO{}, s.handleOrderNotFoundError(err)
	}

	isAdminOrMod := user.UserLevel == RoleAdmin || user.UserLevel == RoleModerator
	if order.OrderUserID != user.UserID && !isAdminOrMod {
		return v1dto.OrderTimelineResponseDTO{}, utils.NewError(utils.ErrCodeForbidden, "Access denied")
	}

	events, err := s.trackingRepo.GetByOrderID(ctx, order.OrderID)
	if err != nil {
		return v1dto.OrderTimelineResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get tracking events", err)
	}

	eventDTOs := v1dto.MapToEventDTOs(events)
	if len(eventDTOs) == 0 {
		eventDTOs = s.synthesizeBaselineTimeline(order)
	}

	return v1dto.OrderTimelineResponseDTO{
		OrderUUID:   order.OrderUuid.String(),
		OrderNumber: order.OrderNumber,
		Events:      eventDTOs,
	}, nil
}

func (s *orderShippingService) synthesizeBaselineTimeline(order sqlc.GetOrderByUUIDRow) []v1dto.TrackingEventDTO {
	eventDTOs := make([]v1dto.TrackingEventDTO, 0)
	var nextID int32 = 1

	// 1. Order Placed
	placedNote := fmt.Sprintf("Order #%s placed successfully by customer", order.OrderNumber)
	eventDTOs = append(eventDTOs, v1dto.TrackingEventDTO{
		EventID:    nextID,
		Status:     "placed",
		StatusText: "Order placed successfully",
		Note:       &placedNote,
		Source:     "system",
		CreatedAt:  order.OrderCreatedAt,
	})
	nextID++

	// 2. Payment confirmed / Order confirmed
	if order.OrderPaymentStatus == 2 || order.OrderStatus >= 2 {
		confTime := order.OrderCreatedAt.Add(time.Minute * 2)
		if order.OrderStatus >= 2 && !order.OrderUpdatedAt.IsZero() && order.OrderUpdatedAt.After(order.OrderCreatedAt) {
			confTime = order.OrderUpdatedAt
		}
		confNote := "Order payment confirmed"
		if order.OrderPaymentMethod == 1 {
			confNote = "Order verified for Cash on Delivery (COD)"
		} else if order.OrderPaymentMethod == 2 {
			confNote = "Bank transfer payment verified"
		} else if order.OrderPaymentMethod == 3 {
			confNote = "E-wallet payment processed successfully"
		}
		eventDTOs = append(eventDTOs, v1dto.TrackingEventDTO{
			EventID:    nextID,
			Status:     "confirmed",
			StatusText: "Payment confirmed",
			Note:       &confNote,
			Source:     "system",
			CreatedAt:  confTime,
		})
		nextID++
	}

	// 3. Shipped / In Transit
	if order.OrderStatus >= 3 || (order.OrderShippingCode != nil && strings.TrimSpace(*order.OrderShippingCode) != "") {
		provider := "GHN Express"
		if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
			provider = *order.OrderShippingProvider
		}
		codeStr := ""
		if order.OrderShippingCode != nil && strings.TrimSpace(*order.OrderShippingCode) != "" {
			codeStr = fmt.Sprintf(" (Tracking: %s)", *order.OrderShippingCode)
		}
		shipNote := fmt.Sprintf("Package handed over to carrier %s%s", provider, codeStr)
		shipTime := order.OrderCreatedAt.Add(time.Hour * 2)
		if !order.OrderUpdatedAt.IsZero() && order.OrderUpdatedAt.After(order.OrderCreatedAt) {
			shipTime = order.OrderUpdatedAt
		}
		eventDTOs = append(eventDTOs, v1dto.TrackingEventDTO{
			EventID:    nextID,
			Status:     "in_transit",
			StatusText: "In transit with carrier",
			Note:       &shipNote,
			Source:     "carrier",
			CreatedAt:  shipTime,
		})
		nextID++
	}

	// 4. Delivered
	if order.OrderStatus == 4 || order.OrderDeliveredAt.Valid {
		delTime := order.OrderCreatedAt.Add(time.Hour * 24)
		if order.OrderDeliveredAt.Valid {
			delTime = order.OrderDeliveredAt.Time
		} else if !order.OrderUpdatedAt.IsZero() && order.OrderUpdatedAt.After(order.OrderCreatedAt) {
			delTime = order.OrderUpdatedAt
		}
		delNote := "Parcel successfully delivered to recipient"
		eventDTOs = append(eventDTOs, v1dto.TrackingEventDTO{
			EventID:    nextID,
			Status:     "delivered",
			StatusText: "Delivered successfully",
			Note:       &delNote,
			Source:     "carrier",
			CreatedAt:  delTime,
		})
		nextID++
	}

	// 5. Cancelled
	if order.OrderStatus == 5 || order.OrderCancelledAt.Valid {
		cancelTime := order.OrderUpdatedAt
		if order.OrderCancelledAt.Valid {
			cancelTime = order.OrderCancelledAt.Time
		}
		cancelNote := "Order has been cancelled"
		eventDTOs = append(eventDTOs, v1dto.TrackingEventDTO{
			EventID:    nextID,
			Status:     "cancelled",
			StatusText: "Order cancelled",
			Note:       &cancelNote,
			Source:     "system",
			CreatedAt:  cancelTime,
		})
		nextID++
	}

	// 6. Returned
	if order.OrderStatus == 6 || order.OrderReturnedAt.Valid {
		retTime := order.OrderUpdatedAt
		if order.OrderReturnedAt.Valid {
			retTime = order.OrderReturnedAt.Time
		}
		retNote := "Return request approved and goods processed"
		eventDTOs = append(eventDTOs, v1dto.TrackingEventDTO{
			EventID:    nextID,
			Status:     "returned",
			StatusText: "Returned goods processed successfully",
			Note:       &retNote,
			Source:     "system",
			CreatedAt:  retTime,
		})
		nextID++
	}

	return eventDTOs
}

// CalculateShippingFee calculates delivery charges based on package weight and destination.
func (s *orderShippingService) CalculateShippingFee(ctx context.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error) {
	provider, err := s.shippingFactory.GetProvider(input.Provider)
	if err != nil {
		return v1dto.ShippingFeeResponseDTO{}, utils.NewError(
			utils.ErrCodeBadRequest,
			fmt.Sprintf("Unsupported shipping provider: %s", input.Provider),
		)
	}

	req := shipping.CreateOrderRequest{
		ToDistrictID: input.DistrictID,
		ToWardCode:   input.WardCode,
		Weight:       input.Weight,
		Length:       input.Length,
		Width:        input.Width,
		Height:       input.Height,
		CODAmount:    input.InsuranceValue,
	}

	fee, err := provider.CalculateFee(ctx, req)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", input.Provider).
			Int("district_id", input.DistrictID).
			Str("ward_code", input.WardCode).
			Msg("Failed to calculate shipping fee from provider")
		return v1dto.ShippingFeeResponseDTO{}, utils.WrapError(
			utils.ErrCodeInternal,
			"Failed to calculate shipping fee from shipping provider",
			err,
		)
	}

	return v1dto.ShippingFeeResponseDTO{
		Provider:    input.Provider,
		ShippingFee: fee,
	}, nil
}

// GetPrintBill retrieves printable thermal label URLs/tokens from carrier.
func (s *orderShippingService) GetPrintBill(ctx context.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error) {
	order, err := s.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return v1dto.PrintBillResponseDTO{}, s.handleOrderNotFoundError(err)
	}

	if order.OrderShippingCode == nil || strings.TrimSpace(*order.OrderShippingCode) == "" {
		return v1dto.PrintBillResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Order does not have a valid shipping code")
	}

	shippingCode := *order.OrderShippingCode
	providerName := "ghn"
	if order.OrderShippingProvider != nil && strings.TrimSpace(*order.OrderShippingProvider) != "" {
		providerName = *order.OrderShippingProvider
	}

	provider, err := s.shippingFactory.GetProvider(providerName)
	if err != nil {
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get shipping provider", err)
	}

	res, err := provider.GetPrintBillURL(ctx, shippingCode)
	if err != nil {
		logger.Log.Error().
			Err(err).
			Str("provider", providerName).
			Str("shipping_code", shippingCode).
			Msg("Failed to get print bill URL from provider")
		return v1dto.PrintBillResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get print bill URL from provider", err)
	}

	return v1dto.PrintBillResponseDTO{
		OrderUUID:    orderUUID.String(),
		OrderNumber:  order.OrderNumber,
		ShippingCode: shippingCode,
		Token:        res.Token,
		PrintA5:      res.PrintA5,
		Print80x80:   res.Print80,
		Print52x70:   res.Print52,
	}, nil
}

// Location proxy methods delegating to LocationService
func (s *orderShippingService) GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error) {
	if s.locationService != nil {
		return s.locationService.GetProvinces(ctx)
	}
	return NewLocationService(s.shippingFactory, s.cache).GetProvinces(ctx)
}

func (s *orderShippingService) GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error) {
	if s.locationService != nil {
		return s.locationService.GetDistricts(ctx, provinceID)
	}
	return NewLocationService(s.shippingFactory, s.cache).GetDistricts(ctx, provinceID)
}

func (s *orderShippingService) GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error) {
	if s.locationService != nil {
		return s.locationService.GetWards(ctx, districtID)
	}
	return NewLocationService(s.shippingFactory, s.cache).GetWards(ctx, districtID)
}
