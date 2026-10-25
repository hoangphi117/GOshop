package repository

import (
	"context"
	"goshop/internal/db/sqlc"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserReader provides read-only access to user entities for peripheral domains.
type UserReader interface {
	GetByID(ctx context.Context, id int32) (sqlc.User, error)
	GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetActiveAdminUserIDs(ctx context.Context) ([]int32, error)
}

type UserRepository interface {
	UserReader
	GetByIDs(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error)
	GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error)
	CountUsers(ctx context.Context, search string, deleted bool) (int64, error)
	Create(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Restore(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	Trash(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetByEmail(ctx context.Context, email string) (sqlc.User, error)
	UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
	GetByOAuth(ctx context.Context, provider, oauthID string) (sqlc.User, error)
	CreateOAuthUser(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error)
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

type AddressRepository interface {
	BeginTx(ctx context.Context) (pgx.Tx, error)
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
	GetLowStockProductsCount(ctx context.Context, threshold int32) (int64, error)
	GetPaymentMethodBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetPaymentMethodBreakdownRow, error)
	GetCartAbandonmentAndFunnelStats(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetCartAbandonmentAndFunnelStatsRow, error)
	GetReturnAnalyticsSummary(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetReturnAnalyticsSummaryRow, error)
	GetReturnReasonsBreakdown(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetReturnReasonsBreakdownRow, error)
	GetTopReturnedProducts(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetTopReturnedProductsRow, error)
	GetCategorySalesBreakdown(ctx context.Context, fromDate, toDate time.Time) ([]sqlc.GetCategorySalesBreakdownRow, error)
	GetCouponPerformanceSummary(ctx context.Context, fromDate, toDate time.Time) (sqlc.GetCouponPerformanceSummaryRow, error)
	GetCouponCampaignBreakdown(ctx context.Context, fromDate, toDate time.Time, limit int32) ([]sqlc.GetCouponCampaignBreakdownRow, error)
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
