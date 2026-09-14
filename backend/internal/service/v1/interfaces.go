package v1service

import (
	"context"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type UserService interface {
	GetAllUsers(ctx *gin.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.User, int32, error)
	CreateUsers(ctx *gin.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetUserByUUID(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error)
	UpdateUser(ctx *gin.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	GetProfile(ctx *gin.Context) (sqlc.User, error)
	UpdateProfile(ctx *gin.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	ChangePassword(ctx *gin.Context, input v1dto.ChangePasswordRequest) error
	SoftDeleteUser(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error)
	RestoreUser(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error)
	TrashUser(ctx *gin.Context, uuid uuid.UUID) error
}

type AuthService interface {
	Register(ctx *gin.Context, input v1dto.RegisterInput) error
	Login(ctx *gin.Context, email, password string) (string, string, int, error)
	Logout(ctx *gin.Context, refreshToken string) error
	RefreshToken(ctx *gin.Context, refreshToken string) (string, string, int, error)
	RequestForgotPassword(ctx *gin.Context, email string) error
	ResetPassword(ctx *gin.Context, token, newPassword string) error
}

type CategoryService interface {
	CreateCategory(ctx *gin.Context, input sqlc.CreateCategoryParams) (sqlc.Category, error)
	GetAllCategories(ctx *gin.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.Category, int32, error)
	GetCategoryByUUID(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error)
	UpdateCategory(ctx *gin.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error)
	SoftDeleteCategory(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error)
	RestoreCategory(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error)
	TrashCategory(ctx *gin.Context, uuid uuid.UUID) error
}

type ProductService interface {
	CreateProduct(ctx *gin.Context, input v1dto.CreateProductInput) (sqlc.CreateProductRow, error)
	GetAllProducts(ctx *gin.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.GetProductByUUIDRow, int32, error)
	GetProductByUUID(ctx *gin.Context, productUuid uuid.UUID) (sqlc.GetProductByUUIDRow, error)
	UpdateProduct(ctx *gin.Context, input v1dto.UpdateProductInput) (sqlc.UpdateProductRow, error)
	SoftDeleteProduct(ctx *gin.Context, uuid uuid.UUID) (sqlc.Product, error)
	RestoreProduct(ctx *gin.Context, uuid uuid.UUID) (sqlc.Product, error)
	TrashProduct(ctx *gin.Context, uuid uuid.UUID) error
}

type OrderService interface {
	CreateOrder(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetMyOrders(ctx context.Context, userUUID uuid.UUID, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderByUUID(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetAllOrders(ctx context.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	UpdateOrderStatus(ctx context.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	CancelOrder(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, reason *string) (sqlc.Order, error)
	UpdateOrderShippingInfo(ctx context.Context, userUUID uuid.UUID, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	UpdateOrderPaymentStatus(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetOrderInvoicePDF(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) ([]byte, string, error)

	// Shipping
	CreateShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CreateShippingInput) (v1dto.AsyncShippingResponseDTO, error)
	ProcessShippingOrderMessage(headers amqp.Table, body []byte) error
	GetOrderTimeline(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.OrderTimelineResponseDTO, error)
	HandleShippingWebhook(ctx context.Context, rawBody []byte, payload v1dto.GHNWebhookPayload) error
	CalculateShippingFee(ctx context.Context, input v1dto.CalculateShippingFeeInput) (v1dto.ShippingFeeResponseDTO, error)
	CancelShipping(ctx context.Context, orderUUID uuid.UUID, input v1dto.CancelShippingInput) (v1dto.CancelShippingResponseDTO, error)
	ConfirmOrderDelivery(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.ConfirmDeliveryResponseDTO, error)
	GetPrintBill(ctx context.Context, orderUUID uuid.UUID) (v1dto.PrintBillResponseDTO, error)
	GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error)
	GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error)
	GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error)

	// Returns & Refunds
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
	SyncOrderTracking(ctx context.Context, userUUID uuid.UUID, orderUUID uuid.UUID) (v1dto.SyncTrackingResponseDTO, error)
	SyncActiveShippingOrders(ctx context.Context) (v1dto.SyncActiveShippingStatsDTO, error)
	AdminSyncActiveShippingOrders(ctx context.Context, userUUID uuid.UUID) (v1dto.SyncActiveShippingStatsDTO, error)
}

type OrderPricingService interface {
	CalculateItems(items []ItemPriceInput) ([]ItemPriceResult, float64, error)
	ValidateAndCalculateDiscount(coupon *sqlc.Coupon, subtotal float64, now time.Time) (float64, error)
	CalculateOrderTotals(ctx context.Context, items []ItemPriceInput, couponCode *string, userID int32,
		shippingFee float64, now time.Time) (*OrderPricingResult, error)
}

type InventoryService interface {

	// LockAndDeductStock locks pessimistic products by UUID order (to avoid deadlock),
	// Checks active status and inventory quantity, then subtracts from stock.
	LockAndDeductStock(ctx context.Context, items []DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error)
	RestoreStock(ctx context.Context, items []RestoreItem) error
}

type CartService interface {
	GetMyCart(ctx *gin.Context) (v1dto.CartDTO, error)
	AddItemToCart(ctx *gin.Context, input v1dto.AddCartItemInput) (v1dto.CartDTO, error)
	UpdateCartItemQuantity(ctx *gin.Context, itemUUID uuid.UUID, input v1dto.UpdateCartItemQuantityInput) (v1dto.CartDTO, error)
	RemoveItemFromCart(ctx *gin.Context, itemUUID uuid.UUID) (v1dto.CartDTO, error)
	ClearMyCart(ctx *gin.Context) error
}

type CouponService interface {
	CreateCoupon(ctx *gin.Context, input v1dto.CreateCouponInput) (v1dto.CouponDTO, error)
	GetAllCoupons(ctx *gin.Context, search, orderBy, sort string, coupon_type *int32, isActive *bool, page, limit int32, deleted bool) ([]v1dto.CouponDTO, int32, error)
	GetCouponByUUID(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	UpdateCoupon(ctx *gin.Context, couponUUID uuid.UUID, input v1dto.UpdateCouponInput) (v1dto.CouponDTO, error)
	SoftDeleteCoupon(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	RestoreCoupon(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	TrashCoupon(ctx *gin.Context, couponUUID uuid.UUID) (v1dto.CouponDTO, error)
	PreviewCoupon(ctx *gin.Context, input v1dto.PreviewCouponInput) (v1dto.PreviewCouponDTO, error)
}

type PaymentService interface {
	CreatePaymentIntent(ctx *gin.Context, input v1dto.CreatePaymentIntentInput) (v1dto.PaymentIntentDTO, error)
	HandleWebhook(ctx *gin.Context, provider string) error
	ProcessWebhookMessage(headers amqp.Table, body []byte) error
	GetPaymentHistory(ctx *gin.Context, orderUUID uuid.UUID) ([]v1dto.PaymentTransactionDTO, error)
	ConfirmCODPayment(ctx *gin.Context, orderUUID uuid.UUID) error
	RefundPayment(ctx *gin.Context, input v1dto.RefundPaymentInput) (v1dto.PaymentTransactionDTO, error)
	ReconcilePendingPayments(ctx context.Context) error
}

type AddressService interface {
	CreateUserAddress(ctx *gin.Context, input v1dto.CreateAddressInput) (v1dto.AddressDTO, error)
	GetMyAddresses(ctx *gin.Context) ([]v1dto.AddressDTO, error)
	GetAddressByUUID(ctx *gin.Context, addressUUID uuid.UUID) (v1dto.AddressDTO, error)
	UpdateUserAddress(ctx *gin.Context, addressUUID uuid.UUID, input v1dto.UpdateAddressInput) (v1dto.AddressDTO, error)
	SetDefaultUserAddress(ctx *gin.Context, addressUUID uuid.UUID) error
	DeleteUserAddress(ctx *gin.Context, addressUUID uuid.UUID) error
}

type ReviewService interface {
	CreateReview(ctx *gin.Context, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error)
	GetProductReviews(ctx *gin.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error)
	GetProductRatingSummary(ctx *gin.Context, productUUID uuid.UUID) (v1dto.RatingSummaryRow, error)
	GetMyReviews(ctx *gin.Context, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error)
	UpdateReview(ctx *gin.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error)
	UpdateReviewStatus(ctx *gin.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error)
	DeleteReview(ctx *gin.Context, reviewUUID uuid.UUID) error
	AdminGetAllReviews(ctx *gin.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewRow, int32, error)
	AdminReplyReview(ctx *gin.Context, reviewUUID uuid.UUID, input v1dto.ReplyReviewInput) (v1dto.ReviewDTO, error)
	ToggleHelpfulVote(ctx *gin.Context, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error)
	ReportReview(ctx *gin.Context, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error
	AdminForceDeleteReview(ctx *gin.Context, reviewUUID uuid.UUID) error
}

type NotificationService interface {
	Send(ctx context.Context, input v1dto.SendNotificationInput) error
	CreateNotification(ctx *gin.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error)
	BroadcastNotification(ctx *gin.Context, input v1dto.BroadcastNotificationInput) error
	GetMyNotifications(ctx *gin.Context, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error)
	GetUnreadNotificationCount(ctx *gin.Context) (v1dto.UnreadCountDTO, error)
	MarkNotificationAsRead(ctx *gin.Context, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error)
	MarkAllNotificationsAsRead(ctx *gin.Context) error
	DeleteNotification(ctx *gin.Context, notificationUUID uuid.UUID) error
}

type BannerService interface {
	GetActiveBanners(ctx *gin.Context, position string) ([]v1dto.BannerDTO, error)
	AdminGetBanners(ctx *gin.Context, query v1dto.AdminGetBannersQuery) ([]v1dto.AdminBannerDetailDTO, int64, error)
	AdminGetBannerByUUID(ctx *gin.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error)
	AdminCreateBanner(ctx *gin.Context, input v1dto.CreateBannerRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminUpdateBanner(ctx *gin.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminUpdateBannerStatus(ctx *gin.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerStatusRequest) (v1dto.AdminBannerDetailDTO, error)
	AdminSoftDeleteBanner(ctx *gin.Context, bannerUUID uuid.UUID) error
	AdminRestoreBanner(ctx *gin.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error)
	AdminTrashBanner(ctx *gin.Context, bannerUUID uuid.UUID) error
}
