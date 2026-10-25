package v1service

import (
	"context"
	v1dto "goshop/internal/dto/v1"

	"github.com/google/uuid"
)

const (
	PaymentStatusPending  int32 = 1
	PaymentStatusPaid     int32 = 2
	PaymentStatusFailed   int32 = 3
	PaymentStatusExpired  int32 = 4
	PaymentStatusRefunded int32 = 5
)

type PaymentRefundService interface {
	RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error)
}

type PaymentService interface {
	PaymentRefundService
	CreatePaymentIntent(ctx context.Context, userID int32, clientIP, idempotencyKey string, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error)
	ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error
}

type AnalyticsService interface {
	GetOverview(ctx context.Context, query v1dto.OverviewStatsQuery) (v1dto.OverviewStatsDTO, error)
	GetRevenueChart(ctx context.Context, query v1dto.RevenueChartQuery) ([]v1dto.RevenueChartPointDTO, error)
	GetRecentOrders(ctx context.Context, limit int32) ([]v1dto.RecentOrderDTO, error)
	GetTopProducts(ctx context.Context, query v1dto.TopProductsQuery) ([]v1dto.TopProductDTO, error)
	GetOrderStatusDistribution(ctx context.Context, params v1dto.OrderStatusDistributionQuery) (v1dto.OrderStatusDistributionResponse, error)
	GetLowStockProducts(ctx context.Context, params v1dto.LowStockProductsQuery) (v1dto.LowStockProductResponse, error)
	GetPaymentMethodBreakdown(ctx context.Context, params v1dto.PaymentMethodBreakdownQuery) (v1dto.PaymentMethodBreakdownResponse, error)
	GetCartFunnel(ctx context.Context, params v1dto.CartFunnelQuery) (*v1dto.CartFunnelDTO, error)
	GetReturnAnalytics(ctx context.Context, params v1dto.ReturnAnalyticsQuery) (*v1dto.ReturnAnalyticsResponse, error)
	GetCategorySalesBreakdown(ctx context.Context, params v1dto.CategorySalesBreakdownQuery) (*v1dto.CategorySalesBreakdownResponse, error)
	GetCouponPerformance(ctx context.Context, params v1dto.CouponPerformanceQuery) (*v1dto.CouponPerformanceResponse, error)
}

type ChatService interface {
	GetOrCreateConversation(ctx context.Context, userUUID uuid.UUID) (*v1dto.ConversationDTO, error)
	SendMessage(ctx context.Context, conversationUUID uuid.UUID, senderUUID uuid.UUID, senderRole string, req v1dto.SendMessageRequest) (v1dto.MessageResponse, error)
	ListMessages(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID, req v1dto.ListMessagesRequest) ([]v1dto.MessageResponse, int64, error)
	MarkRead(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID) error
	ListConversations(ctx context.Context, callerUUID uuid.UUID, req v1dto.ListConversationsRequest) ([]v1dto.ConversationResponse, int64, error)
	GetConversationDetails(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID) (v1dto.ConversationResponse, error)
	CloseConversation(ctx context.Context, conversationUUID uuid.UUID) (*v1dto.ConversationDTO, error)
	ReopenConversation(ctx context.Context, conversationUUID uuid.UUID) (*v1dto.ConversationDTO, error)
	GetTotalUnreadCount(ctx context.Context, callerUUID uuid.UUID) (int64, error)
	SendTyping(ctx context.Context, conversationUUID uuid.UUID, callerUUID uuid.UUID, isTyping bool) error
	RecallMessage(ctx context.Context, messageUUID uuid.UUID, callerUUID uuid.UUID) (v1dto.MessageResponse, error)
	GetShopStatus(ctx context.Context) (v1dto.ShopStatusResponse, error)
}
