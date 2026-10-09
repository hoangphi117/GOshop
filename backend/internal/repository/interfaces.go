package repository

import (
	"context"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error)
	CountUsers(ctx context.Context, search string, deleted bool) (int64, error)
	Create(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetActiveAdminUserIDs(ctx context.Context) ([]int32, error)
	Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetByEmail(ctx context.Context, email string) (sqlc.User, error)
	UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
	GetByOAuth(ctx context.Context, provider, oauthID string) (sqlc.User, error)
	CreateOAuthUser(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error)
}

type CategoryRepository interface {
	Create(ctx context.Context, input sqlc.CreateCategoryParams) (sqlc.Category, error)
	CountCategories(ctx context.Context, search string, deleted bool) (int64, error)
	GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error)
	GetBySlug(ctx context.Context, slug string) (sqlc.Category, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	Update(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Category, error)
}

type ProductRepository interface {
	Create(ctx context.Context, productParams sqlc.CreateProductParams) (sqlc.CreateProductRow, error)
	CountProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, deleted bool) (int64, error)
	GetAll(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error)
	GetBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetProductByUUIDRow, error)
	GetByUUIDs(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error)
	Update(ctx context.Context, productParams sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Product, error)
	DeductStock(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error
	RestoreStock(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error
}

type OrderRepository interface {
	Create(ctx context.Context, params sqlc.CreateOrderParams) (sqlc.Order, error)
	GetLastOrderNumber(ctx context.Context, orderNumber string) (string, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.GetOrderByUUIDRow, error)
	GetByID(ctx context.Context, orderID int32) (sqlc.GetOrderByIDRow, error)
	GetByUUIDForUpdate(ctx context.Context, tx pgx.Tx, uuid uuid.UUID) (sqlc.Order, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (sqlc.GetOrderByOrderNumberRow, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderStatusParams) (sqlc.Order, error)
	CancelOrder(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, params sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, error)
	UpdateOrderPaymentStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateOrderPaymentStatusParams) (sqlc.Order, error)
	CountOrders(ctx context.Context, userID, status *int32, search string) (int64, error)
	GetAll(ctx context.Context, userID, status *int32, search, orderBy, sort string, limit, offset int32) ([]sqlc.Order, error)

	// Shipping
	UpdateShippingAndStatus(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID, params sqlc.UpdateOrderShippingAndStatusParams) (sqlc.Order, error)
	GetByShippingCode(ctx context.Context, shippingCode string) (sqlc.GetOrderByShippingCodeRow, error)
	MarkDeliveredByShippingCode(ctx context.Context, tx pgx.Tx, shippingCode string) (sqlc.Order, error)
	MarkDeliveredByUUID(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	CancelShipping(ctx context.Context, tx pgx.Tx, orderUUID uuid.UUID) (sqlc.Order, error)
	GetActiveShippedOrders(ctx context.Context, limit int32) ([]sqlc.Order, error)
}

type OrderReturnsRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateOrderReturnParams) (sqlc.OrderReturn, error)
	GetActiveByOrderID(ctx context.Context, orderID int32) (sqlc.OrderReturn, error)
	GetLatestByOrderID(ctx context.Context, orderID int32) (sqlc.OrderReturn, error)
	GetReturns(ctx context.Context, params sqlc.AdminGetReturnsParams) ([]sqlc.AdminGetReturnsRow, error)
	GetByUUID(ctx context.Context, returnUUID uuid.UUID) (sqlc.OrderReturn, error)
	CountReturns(ctx context.Context, params sqlc.AdminCountReturnsParams) (int64, error)
	GetReturnsByUserID(ctx context.Context, params sqlc.GetReturnsByUserIDParams) ([]sqlc.GetReturnsByUserIDRow, error)
	CountReturnsByUserID(ctx context.Context, params sqlc.CountReturnsByUserIDParams) (int64, error)
	AdminGetReturnByUUID(ctx context.Context, returnUUID uuid.UUID) (sqlc.AdminGetReturnByUUIDRow, error)
	AdminGetLatestReturnByOrderID(ctx context.Context, orderID int32) (sqlc.AdminGetLatestReturnByOrderIDRow, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnStatusParams) (sqlc.OrderReturn, error)
	UpdateReturnShippingInfo(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnShippingInfoParams) (sqlc.OrderReturn, error)
	UpdateReturnBankAccount(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReturnBankAccountParams) (sqlc.OrderReturn, error)
}

type TrackingRepository interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, params sqlc.InsertTrackingEventParams) (sqlc.OrderTrackingEvent, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.OrderTrackingEvent, error)
}

type OrderItemRepository interface {
	Create(ctx context.Context, tx pgx.Tx, input sqlc.CreateOrderItemParams) (sqlc.OrderItem, error)
	GetByOrderID(ctx context.Context, orderID int32) ([]sqlc.GetOrderItemsByOrderIDRow, error)
}

type CartRepository interface {
	GetOrCreateCart(ctx context.Context, userID int32) (sqlc.Cart, error)
	GetCartByUserID(ctx context.Context, userID int32) (sqlc.Cart, error)
	GetCartItems(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error)
	AddOrUpdateItem(ctx context.Context, params sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error)
	GetCartItemQuantity(ctx context.Context, cartID, productID int32) (int32, error)
	SetCartItemQuantity(ctx context.Context, params sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error)
	DeleteItem(ctx context.Context, cartID int32, itemUUID uuid.UUID) error
	ClearCart(ctx context.Context, cartID int32) error
	DeleteCartItemsByProductIDs(ctx context.Context, tx pgx.Tx, cartID int32, productIDs []int32) error
}

type CouponRepository interface {
	Create(ctx context.Context, params sqlc.CreateCouponParams) (sqlc.Coupon, error)
	GetAll(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, limit, offset int32, deleted bool) ([]sqlc.Coupon, error)
	GetByCode(ctx context.Context, code string) (sqlc.Coupon, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	Update(ctx context.Context, params sqlc.UpdateCouponParams) (sqlc.Coupon, error)
	CountCoupons(ctx context.Context, search string, coupon_type *int32, isActive *bool, deleted bool) (int64, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.Coupon, error)
	CountUserUsages(ctx context.Context, userID, couponID int32) (int64, error)

	RecordUserUsage(ctx context.Context, tx pgx.Tx, params sqlc.RecordCouponUsageParams) (sqlc.CouponUsage, error)
	IncrementUsedCount(ctx context.Context, tx pgx.Tx, couponID int32) error
}

type PaymentRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreatePaymentTransactionParams) (sqlc.PaymentTransaction, error)
	GetByIdempotencyKey(ctx context.Context, tx pgx.Tx, idempotencyKey string) (sqlc.PaymentTransaction, error)
	GetByTxnRef(ctx context.Context, tx pgx.Tx, provider, txnRef string) (sqlc.PaymentTransaction, error)
	GetByProviderTxnID(ctx context.Context, tx pgx.Tx, provider, providerTxnID string) (sqlc.PaymentTransaction, error)
	GetByOrderID(ctx context.Context, tx pgx.Tx, orderID int32) ([]sqlc.PaymentTransaction, error)
	UpdateStatusAtomically(ctx context.Context, tx pgx.Tx, params sqlc.UpdatePaymentStatusAtomicallyParams) (sqlc.PaymentTransaction, error)
	CreateRefund(ctx context.Context, tx pgx.Tx, params sqlc.CreateRefundTransactionParams) (sqlc.PaymentTransaction, error)
	GetPendingTransactionsOlderThan(ctx context.Context, tx pgx.Tx, params sqlc.GetPendingTransactionsOlderThanParams) ([]sqlc.GetPendingTransactionsOlderThanRow, error)
}

type WebhookEventRepository interface {
	InsertEvent(ctx context.Context, tx pgx.Tx, params sqlc.InsertWebhookEventParams) (sqlc.WebhookEvent, error)
	MarkProcessed(ctx context.Context, tx pgx.Tx, eventID int32) error
}

type AddressRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateUserAddressParams) (sqlc.UserAddress, error)
	GetByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error)
	GetByUserID(ctx context.Context, userID int32) ([]sqlc.UserAddress, error)
	ClearDefault(ctx context.Context, tx pgx.Tx, userID int32) error
	Update(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error)
	Count(ctx context.Context, userID int32) (int64, error)
	SetDefault(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error)
	Delete(ctx context.Context, tx pgx.Tx, userID int32, addressUUID uuid.UUID) (bool, error)
	SetLatestAsDefault(ctx context.Context, tx pgx.Tx, userID int32) error
}

type ReviewRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateReviewParams) (sqlc.ProductReview, error)
	CheckPurchased(ctx context.Context, orderID, userID, productID int32) (bool, error)
	CheckExists(ctx context.Context, userID, productID, orderID int32) (bool, error)
	RecalcRating(ctx context.Context, tx pgx.Tx, productID int32) error
	GetByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]v1dto.ReviewRow, error)
	CountByProduct(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error)
	GetRatingSummary(ctx context.Context, productID int32) (sqlc.GetProductRatingSummaryRow, error)
	GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.GetReviewsByUserIDRow, error)
	CountByUserID(ctx context.Context, userID int32) (int64, error)
	GetByUUID(ctx context.Context, reviewUUID uuid.UUID) (sqlc.GetReviewByUUIDRow, error)
	Update(ctx context.Context, tx pgx.Tx, params sqlc.UpdateReviewParams) (sqlc.ProductReview, error)
	Delete(ctx context.Context, tx pgx.Tx, reviewID int32) error

	// Report
	HasReported(ctx context.Context, userID, reviewID int32) (bool, error)
	CreateReport(ctx context.Context, tx pgx.Tx, params sqlc.InsertReviewReportParams) error
	CountReports(ctx context.Context, tx pgx.Tx, reviewID int32) (int64, error)
	GetReportsByReviewID(ctx context.Context, reviewID int32) ([]sqlc.GetReviewReportsByReviewIDRow, error)
	DeleteReportsByReviewID(ctx context.Context, tx pgx.Tx, reviewID int32) error

	// Helpful votes
	HasHelpfulVote(ctx context.Context, userID, reviewID int32) (bool, error)
	AddHelpfulVote(ctx context.Context, tx pgx.Tx, userID, reviewID int32) (int32, error)
	RemoveHelpfulVote(ctx context.Context, tx pgx.Tx, userID, reviewID int32) (int32, error)

	// Admin
	AdminGetAll(ctx context.Context, status *int16, rating *int32, productUUID, search *string, orderBy, sort string, limit, offset int32) ([]v1dto.AdminReviewRow, error)
	AdminCount(ctx context.Context, status *int16, rating *int32, productID *int32, search *string) (int64, error)
	UpdateStatus(ctx context.Context, tx pgx.Tx, reviewID int32, status int16) (sqlc.ProductReview, error)
	Reply(ctx context.Context, tx pgx.Tx, reviewID int32, reply string) (sqlc.ProductReview, error)
}

type NotificationRepository interface {
	Create(ctx context.Context, params sqlc.CreateNotificationParams) (sqlc.Notification, error)
	CreateForAdmins(ctx context.Context, nType, title, body string, data []byte) error
	GetAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool, limit, offset int32) ([]sqlc.Notification, error)
	CountAdminNotifications(ctx context.Context, userID int32, nType *string, isRead *bool) (int64, error)
	CountAdminUnreadNotifications(ctx context.Context, userID int32) (int64, error)
	MarkAllAdminAsRead(ctx context.Context, userID int32) error
	CreateForAllUsers(ctx context.Context, nType, title, body string, data []byte) error
	GetByUserID(ctx context.Context, userID int32, limit, offset int32) ([]sqlc.Notification, error)
	CountByUserID(ctx context.Context, userID int32) (int64, error)
	CountUnreadByUserID(ctx context.Context, userID int32) (int64, error)
	MarkAsRead(ctx context.Context, notifUUID uuid.UUID, userID int32) (sqlc.Notification, error)
	MarkAllAsRead(ctx context.Context, userID int32) error
	Delete(ctx context.Context, notifUUID uuid.UUID, userID int32) error
	DeleteOldReadNotifications(ctx context.Context, cutoff time.Time) (int64, error)
}

type BannerRepository interface {
	GetActiveByPosition(ctx context.Context, position string) ([]sqlc.GetActiveBannersByPositionRow, error)
	Create(ctx context.Context, params sqlc.CreateBannerParams) (sqlc.Banner, error)
	GetByUUID(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error)
	AdminList(ctx context.Context, params sqlc.AdminListBannersParams) ([]sqlc.Banner, error)
	AdminCount(ctx context.Context, params sqlc.AdminCountBannersParams) (int64, error)
	Update(ctx context.Context, params sqlc.UpdateBannerParams) (sqlc.Banner, error)
	UpdateStatus(ctx context.Context, bannerUUID uuid.UUID, isActive bool) (sqlc.Banner, error)
	SoftDelete(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error)
	Restore(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error)
	Trash(ctx context.Context, bannerUUID uuid.UUID) error
}

type AnalyticsRepository interface {
	GetOverview(ctx context.Context) (sqlc.GetOverviewStatsRow, error)
	GetOverviewCustom(ctx context.Context, fromDate, toDate, prevFromDate time.Time) (sqlc.GetOverviewStatsCustomRow, error)
	GetRevenueDaily(ctx context.Context, days int32) ([]sqlc.GetRevenueDailyChartRow, error)
	GetRevenueDailyCustom(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetRevenueDailyCustomRow, error)
	GetRevenueMonthly(ctx context.Context, months int32) ([]sqlc.GetRevenueMonthlyChartRow, error)
	GetRecentOrders(ctx context.Context, limit int32) ([]sqlc.GetRecentOrdersRow, error)
	GetTopSellingProducts(ctx context.Context, days, limit int32) ([]sqlc.GetTopSellingProductsRow, error)
	GetTopSellingProductsCustom(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopSellingProductsCustomRow, error)
	GetOrderStatusDistribution(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetOrderStatusDistributionRow, error)
	GetLowStockProducts(ctx context.Context, threshold, limit int32) ([]sqlc.GetLowStockProductsRow, error)
	GetPaymentMethodBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error)
	GetCartAbandonmentAndFunnelStats(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetCartAbandonmentAndFunnelStatsRow, error)
	GetReturnAnalyticsSummary(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetReturnAnalyticsSummaryRow, error)
	GetReturnReasonsBreakdown(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetReturnReasonsBreakdownRow, error)
	GetTopReturnedProducts(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopReturnedProductsRow, error)
	GetCategorySalesBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetCategorySalesBreakdownRow, error)
	GetCouponPerformanceSummary(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetCouponPerformanceSummaryRow, error)
	GetCouponCampaignBreakdown(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetCouponCampaignBreakdownRow, error)
}

type StockMovementRepository interface {
	Create(ctx context.Context, tx pgx.Tx, params sqlc.CreateStockMovementParams) (sqlc.StockMovement, error)
	List(ctx context.Context, params sqlc.ListStockMovementsParams) ([]sqlc.ListStockMovementsRow, error)
	Count(ctx context.Context, params sqlc.CountStockMovementsParams) (int64, error)
}

type ChatRepository interface {
	CreateConversation(ctx context.Context, userID int32) (sqlc.ChatConversation, error)
	GetConversationByUUID(ctx context.Context, conversationUUID uuid.UUID) (sqlc.ChatConversation, error)
	GetConversationByID(ctx context.Context, conversationID int32) (sqlc.ChatConversation, error)
	GetConversationByUserID(ctx context.Context, userID int32) (sqlc.ChatConversation, error)
	UpdateConversationStatus(ctx context.Context, conversationID int32, status string) (sqlc.ChatConversation, error)
	UpdateLastMessageAt(ctx context.Context, conversationID int32) error
	ListConversations(ctx context.Context, params sqlc.ListConversationsParams) ([]sqlc.ListConversationsRow, error)
	CountConversations(ctx context.Context, params sqlc.CountConversationsParams) (int64, error)
	CreateMessage(ctx context.Context, params sqlc.CreateMessageParams) (sqlc.ChatMessage, error)
	ListMessages(ctx context.Context, params sqlc.ListMessagesParams) ([]sqlc.ChatMessage, error)
	ListMessagesCursor(ctx context.Context, params sqlc.ListMessagesCursorParams) ([]sqlc.ChatMessage, error)
	CountMessagesByConversationID(ctx context.Context, conversationID int32) (int64, error)
	MarkMessagesRead(ctx context.Context, conversationID int32, readerID int32) error
	CountUnreadMessages(ctx context.Context, conversationID int32, readerID int32) (int64, error)
	GetTotalUnreadMessagesForUser(ctx context.Context, userID int32) (int64, error)
	GetTotalUnreadMessagesForAdmin(ctx context.Context) (int64, error)
	GetMessageByUUID(ctx context.Context, messageUUID uuid.UUID) (sqlc.ChatMessage, error)
	RecallMessage(ctx context.Context, messageID int32) (sqlc.ChatMessage, error)
}

type OutboxRepository interface {
	Insert(ctx context.Context, tx pgx.Tx, aggregateType, aggregateID, eventType string, payload any) (sqlc.OutboxEvent, error)
	GetPending(ctx context.Context, limit int32) ([]sqlc.OutboxEvent, error)
	GetPendingByAggregateTypes(ctx context.Context, aggregateTypes []string, limit int32) ([]sqlc.OutboxEvent, error)
	MarkProcessed(ctx context.Context, eventIDs []int64) error
	MarkFailed(ctx context.Context, eventID int64, lastErr *string) error
}

