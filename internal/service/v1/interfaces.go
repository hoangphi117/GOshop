package v1service

import (
	"context"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"

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
	CreateOrder(ctx *gin.Context, input v1dto.CreateOrderInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetMyOrders(ctx *gin.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	GetOrderByUUID(ctx *gin.Context, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetAllOrders(ctx *gin.Context, input v1dto.GetOrdersParam) ([]v1dto.OrderSummaryDTO, int32, error)
	UpdateOrderStatus(ctx *gin.Context, input sqlc.UpdateOrderStatusParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	CancelOrder(ctx *gin.Context, orderUUID uuid.UUID) error
	UpdateOrderShippingInfo(ctx *gin.Context, input sqlc.UpdateOrderShippingInfoParams) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	UpdateOrderPaymentStatus(ctx *gin.Context, orderUUID uuid.UUID, input v1dto.UpdateOrderPaymentStatusInput) (sqlc.Order, []sqlc.GetOrderItemsByOrderIDRow, error)
	GetOrderInvoicePDF(ctx *gin.Context, orderUUID uuid.UUID) ([]byte, string, error)
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
	CreateReview(ctx *gin.Context, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewResponseDTO, error)
	GetProductReviews(ctx *gin.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error)
}
