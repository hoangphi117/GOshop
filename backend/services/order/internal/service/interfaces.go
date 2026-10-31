package service

import (
	"context"
	"time"

	v1dto "goshop/services/order/internal/dto"

	"github.com/google/uuid"
)

type CustomerOrderService interface {
	CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (*v1dto.OrderDTO, error)
	GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (*v1dto.OrderDTO, error)
	CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (*v1dto.OrderSummaryDTO, error)
	CancelExpiredOrder(ctx context.Context, orderUUID uuid.UUID) error
	UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderShippingInfoInput) (*v1dto.OrderDTO, error)
	UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (*v1dto.OrderDTO, error)
}

type OrderAdminService interface {
	GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	UpdateOrderStatus(ctx context.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderStatusInput) (*v1dto.OrderDTO, error)
	BulkUpdateStatus(ctx context.Context, input v1dto.BulkUpdateOrderStatusInput) (*v1dto.BulkOperationResultDTO, error)
	BulkCancel(ctx context.Context, userUUID uuid.UUID, input v1dto.BulkCancelOrdersInput) (*v1dto.BulkOperationResultDTO, error)
}

type OrderInvoiceService interface {
	GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error)
}

type OrderService interface {
	CustomerOrderService
	OrderAdminService
	OrderInvoiceService
}

type OrderShippingService interface {
	CreateShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error)
	ProcessShippingOrder(ctx context.Context, msg v1dto.CreateShippingQueueMessage) error
	GetOrderTimeline(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error)
	HandleShippingWebhook(ctx context.Context, rawBody []byte, payload v1dto.GHNWebhookPayload) error
	CalculateShippingFee(ctx context.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error)
	CancelShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error)
	ConfirmOrderDelivery(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error)
	GetPrintBill(ctx context.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error)
	GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error)
	GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error)
	GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error)
	SyncOrderTracking(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error)
	SyncActiveShippingOrders(ctx context.Context) (v1dto.SyncActiveShippingStatsDTO, error)
	AdminSyncActiveShippingOrders(ctx context.Context, userUUID uuid.UUID) (v1dto.SyncActiveShippingStatsDTO, error)
}

type LocationService interface {
	GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error)
	GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error)
	GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error)
}

type OrderReturnService interface {
	RequestOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CreateOrderReturnInput, imageURLs []string, videoURL *string) (v1dto.OrderReturnDTO, error)
	GetOrderReturnDetails(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderReturnDetailDTO, error)
	GetMyOrderReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyOrderReturnsQuery) ([]v1dto.UserReturnSummaryDTO, int64, error)
	AdminGetReturns(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetReturnsQuery) ([]v1dto.AdminReturnSummaryDTO, int64, error)
	AdminGetReturnDetail(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID) (v1dto.AdminReturnDetailDTO, error)
	AdminUpdateReturnStatus(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminUpdateReturnInput) (v1dto.AdminReturnSummaryDTO, error)
	CancelOrderReturn(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.CancelOrderReturnInput) (v1dto.CancelOrderReturnResponseDTO, error)
	SubmitReturnShipping(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.SubmitReturnShippingInput) (v1dto.SubmitReturnShippingResponseDTO, error)
	UpdateReturnBankAccount(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateReturnBankAccountInput) (v1dto.UpdateReturnBankAccountResponseDTO, error)
	AdminProcessReturnRefund(ctx context.Context, userUUID uuid.UUID, returnUUID uuid.UUID, input v1dto.AdminReturnRefundInput) (v1dto.AdminReturnRefundResponseDTO, error)
}

type OrderPricingService interface {
	CalculateItems(items []ItemPriceInput) ([]ItemPriceResult, float64, error)
	CalculateOrderTotals(ctx context.Context, items []ItemPriceInput, couponCode *string, userID int32,
		shippingFee float64, now time.Time) (*OrderPricingResult, error)
}

type CartService interface {
	RemovePurchasedItems(ctx context.Context, userID int32, productIDs []int32) error
}
