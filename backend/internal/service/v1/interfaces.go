package v1service

import (
	"context"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"io"
	"time"

	"github.com/google/uuid"
)

type UserService interface {
	GetAllUsers(ctx context.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.User, int32, error)
	CreateUsers(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error)
	UpdateUser(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	GetProfile(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error)
	UpdateProfile(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateUserParams) (sqlc.User, error)
	ChangePassword(ctx context.Context, userUUID uuid.UUID, input v1dto.ChangePasswordRequest) error
	SoftDeleteUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error)
	RestoreUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error)
	TrashUser(ctx context.Context, userUUID uuid.UUID) error
}

type AuthService interface {
	Register(ctx context.Context, input v1dto.RegisterInput) error
	Login(ctx context.Context, ip string, email, password string) (string, string, int, error)
	Logout(ctx context.Context, accessToken string, refreshToken string) error
	RefreshToken(ctx context.Context, refreshToken string) (string, string, int, error)
	RequestForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
}

type CategoryService interface {
	CreateCategory(ctx context.Context, input sqlc.CreateCategoryParams) (sqlc.Category, error)
	GetAllCategories(ctx context.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.Category, int32, error)
	GetCategoryByUUID(ctx context.Context, categoryUUID uuid.UUID) (sqlc.Category, error)
	UpdateCategory(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error)
	SoftDeleteCategory(ctx context.Context, categoryUUID uuid.UUID) (sqlc.Category, error)
	RestoreCategory(ctx context.Context, categoryUUID uuid.UUID) (sqlc.Category, error)
	TrashCategory(ctx context.Context, categoryUUID uuid.UUID) error
}

type ProductQueryService interface {
	GetAllProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.GetProductByUUIDRow, int32, error)
	GetProductByUUID(ctx context.Context, productUUID uuid.UUID) (sqlc.GetProductByUUIDRow, error)
	GetProductBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error)
}

type AdminProductService interface {
	CreateProduct(ctx context.Context, input v1dto.CreateProductInput) (sqlc.CreateProductRow, error)
	UpdateProduct(ctx context.Context, input v1dto.UpdateProductInput) (sqlc.UpdateProductRow, error)
	SoftDeleteProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error)
	RestoreProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error)
	TrashProduct(ctx context.Context, productUUID uuid.UUID) error
	ImportProductsCSV(ctx context.Context, file io.Reader) (*v1dto.ProductCSVImportResponse, error)
}

type ProductService interface {
	ProductQueryService
	AdminProductService
}

type CustomerOrderService interface {
	CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error)
	CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
}

type OrderAdminService interface {
	GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
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
	ValidateAndCalculateDiscount(coupon *sqlc.Coupon, subtotal float64, now time.Time) (float64, error)
	CalculateOrderTotals(ctx context.Context, items []ItemPriceInput, couponCode *string, userID int32,
		shippingFee float64, now time.Time) (*OrderPricingResult, error)
}

type InventoryService interface {
	LockAndDeductStock(ctx context.Context, items []DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	LockAndDeductStockWithRef(ctx context.Context, items []DeductItem, refID string, refType string, actorID *int32, note *string) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	RestoreStock(ctx context.Context, items []RestoreItem) error
	RestoreStockWithRef(ctx context.Context, items []RestoreItem, refID string, refType string, actorID *int32, note *string) error
	AdjustStock(ctx context.Context, actorUUID uuid.UUID, input v1dto.AdjustStockInput) (sqlc.StockMovement, error)
	ListStockMovements(ctx context.Context, params v1dto.GetStockMovementsParam) ([]v1dto.StockMovementDTO, int64, error)
}

type CartService interface {
	GetMyCart(ctx context.Context, userUUID uuid.UUID) (v1dto.CartDTO, error)
	AddItemToCart(ctx context.Context, userUUID uuid.UUID, input v1dto.AddCartItemInput) (v1dto.CartDTO, error)
	UpdateCartItemQuantity(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error)
	RemoveItemFromCart(ctx context.Context, userUUID uuid.UUID, itemUUID uuid.UUID) (v1dto.CartDTO, error)
	ClearMyCart(ctx context.Context, userUUID uuid.UUID) error
}

type CouponService interface {
	CreateCoupon(ctx context.Context, input v1dto.CreateCouponInput) (v1dto.CouponDTO, error)
	GetAllCoupons(ctx context.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, page, limit int32, deleted bool) ([]v1dto.CouponDTO, int32, error)
	GetCouponByUUID(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	UpdateCoupon(ctx context.Context, couponUUID uuid.UUID, input v1dto.UpdateCouponInput) (v1dto.CouponDTO, error)
	SoftDeleteCoupon(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	RestoreCoupon(ctx context.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	TrashCoupon(ctx context.Context, couponUUID uuid.UUID) error
	PreviewCoupon(ctx context.Context, userUUID uuid.UUID, input v1dto.PreviewCouponInput) (v1dto.PreviewCouponDTO, error)
}

type PaymentRefundService interface {
	RefundPayment(ctx context.Context, idempotencyKey string, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error)
}

type PaymentService interface {
	PaymentRefundService
	CreatePaymentIntent(ctx context.Context, userUUID uuid.UUID, clientIP, idempotencyKey string, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error)
	HandleWebhook(ctx context.Context, provider string, rawPayload []byte, headers map[string]string) error
	ProcessWebhook(ctx context.Context, msg v1dto.PaymentWebhookMessage) error
	GetPaymentHistory(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]v1dto.PaymentTransactionDTO, error)
	ConfirmCODPayment(ctx context.Context, orderUUID uuid.UUID) error
	ReconcilePendingPayments(ctx context.Context) error
}

type AddressService interface {
	CreateUserAddress(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateAddressInput) (v1dto.AddressDTO, error)
	GetMyAddresses(ctx context.Context, userUUID uuid.UUID) ([]v1dto.AddressDTO, error)
	GetAddressByUUID(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) (v1dto.AddressDTO, error)
	UpdateUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID, input v1dto.UpdateAddressInput) (v1dto.AddressDTO, error)
	SetDefaultUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error
	DeleteUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error
}

type ReviewService interface {
	CreateReview(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error)
	GetProductReviews(ctx context.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error)
	GetProductRatingSummary(ctx context.Context, productUUID uuid.UUID) (v1dto.RatingSummaryRow, error)
	GetMyReviews(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error)
	UpdateReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error)
	UpdateReviewStatus(ctx context.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error)
	DeleteReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) error
	AdminGetAllReviews(ctx context.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewDTO, int32, error)
	AdminReplyReview(ctx context.Context, reviewUUID uuid.UUID, input v1dto.ReplyReviewInput) (v1dto.ReviewDTO, error)
	ToggleHelpfulVote(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error)
	ReportReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error
	AdminForceDeleteReview(ctx context.Context, reviewUUID uuid.UUID) error
	GetReviewReports(ctx context.Context, reviewUUID uuid.UUID) ([]v1dto.ReviewReportDTO, error)
	AdminDismissReports(ctx context.Context, reviewUUID uuid.UUID) error
}

type NotificationService interface {
	Send(ctx context.Context, input v1dto.SendNotificationInput) error
	SendAsync(ctx context.Context, input v1dto.SendNotificationInput) error
	CreateNotification(ctx context.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error)
	NotifyAdmins(ctx context.Context, input v1dto.AdminNotifyInput) error
	NotifyAdminsAsync(ctx context.Context, input v1dto.AdminNotifyInput) error
	BroadcastNotification(ctx context.Context, input v1dto.BroadcastNotificationInput) error
	GetMyNotifications(ctx context.Context, userUUID uuid.UUID, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error)
	GetAdminNotifications(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error)
	GetUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (v1dto.UnreadCountDTO, error)
	MarkNotificationAsRead(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error)
	MarkAllNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error
	DeleteNotification(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) error
	PurgeStaleNotifications(ctx context.Context, retentionDays int) (int64, error)
}

type BannerService interface {
	GetActiveBanners(ctx context.Context, position string) ([]v1dto.BannerDTO, error)
	AdminGetBanners(ctx context.Context, query v1dto.AdminGetBannersQuery) ([]v1dto.AdminBannerDetailDTO, int64, error)
	AdminGetBannerByUUID(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error)
	AdminCreateBanner(ctx context.Context, input v1dto.CreateBannerRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminUpdateBanner(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminUpdateBannerStatus(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerStatusRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminSoftDeleteBanner(ctx context.Context, bannerUUID uuid.UUID) error
	AdminRestoreBanner(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error)
	AdminTrashBanner(ctx context.Context, bannerUUID uuid.UUID) error
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
