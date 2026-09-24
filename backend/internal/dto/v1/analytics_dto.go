package v1dto

// ============================================================================
// 1. RESPONSE DTOs
// ============================================================================

type OverviewStatsDTO struct {
	TotalRevenue   float64 `json:"total_revenue"`
	TotalOrders    int64   `json:"total_orders"`
	NewCustomers   int64   `json:"new_customers"`
	PendingOrders  int64   `json:"pending_orders"`
	RevenueGrowth  float64 `json:"revenue_growth"`  // % vs previous 30 days
	OrderGrowth    float64 `json:"order_growth"`    // % vs previous 30 days
	CustomerGrowth float64 `json:"customer_growth"` // % vs previous 30 days
}

type RevenueChartPointDTO struct {
	Date       string  `json:"date"`
	Revenue    float64 `json:"revenue"`
	OrderCount int64   `json:"order_count"`
}

type RecentOrderDTO struct {
	UUID          string  `json:"uuid"`
	OrderNumber   string  `json:"order_number"`
	CustomerName  string  `json:"customer_name"`
	CustomerEmail string  `json:"customer_email"`
	TotalAmount   float64 `json:"total_amount"`
	PaymentMethod int32   `json:"payment_method"`
	PaymentStatus int32   `json:"payment_status"`
	Status        int32   `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

type TopProductDTO struct {
	UUID           string  `json:"uuid"`
	Name           string  `json:"name"`
	Slug           string  `json:"slug"`
	Price          float64 `json:"price"`
	ImageURL       *string `json:"image_url,omitempty"`
	CategoryName   string  `json:"category_name"`
	TotalUnitsSold int64   `json:"total_units_sold"`
	TotalRevenue   float64 `json:"total_revenue"`
}

// OrderStatusDistributionItemDTO represents count and revenue for an individual order status
type OrderStatusDistributionItemDTO struct {
	Status      int32   `json:"status"`       // 1: Pending, 2: Confirmed, 3: Shipped, 4: Delivered, 5: Cancelled, 6: Returned
	StatusName  string  `json:"status_name"`  // e.g., "Pending", "Delivered", "Cancelled"
	Count       int64   `json:"count"`        // Number of orders in this status
	TotalAmount float64 `json:"total_amount"` // Total monetary value of orders in this status
	Percentage  float64 `json:"percentage"`   // Percentage of total orders (0.0 to 100.0)
}

// OrderStatusDistributionResponse represents the aggregated distribution across all statuses
type OrderStatusDistributionResponse struct {
	TotalOrders int64                            `json:"total_orders"`
	Items       []OrderStatusDistributionItemDTO `json:"items"`
}

// LowStockProductDTO represents an inventory alert for low or out-of-stock items
type LowStockProductDTO struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"` // Current available stock
	ImageURL     *string `json:"image_url,omitempty"`
	CategoryName string  `json:"category_name"`
	AlertLevel   string  `json:"alert_level"` // "critical" (stock <= 0), "warning" (stock <= threshold)
}

// LowStockProductResponse represents the list of depleted inventory items
type LowStockProductResponse struct {
	Threshold int32                `json:"threshold"` // Warning threshold used (e.g., 5)
	TotalLow  int64                `json:"total_low"` // Count of items below threshold
	Items     []LowStockProductDTO `json:"items"`
}

// PaymentMethodBreakdownItemDTO represents contribution by a payment channel
type PaymentMethodBreakdownItemDTO struct {
	PaymentMethod int32   `json:"payment_method"` // 1: COD, 2: Online / Bank Transfer, 3: E-Wallet
	MethodName    string  `json:"method_name"`    // "COD (Cash on Delivery)", "Bank Transfer", "E-Wallet"
	OrderCount    int64   `json:"order_count"`    // Total orders paid via this method
	NetRevenue    float64 `json:"net_revenue"`    // Net revenue after refunds
	Percentage    float64 `json:"percentage"`     // Revenue share percentage (0.0 to 100.0)
}

// PaymentMethodBreakdownResponse represents aggregated revenue across payment channels
type PaymentMethodBreakdownResponse struct {
	TotalRevenue float64                         `json:"total_revenue"`
	TotalOrders  int64                           `json:"total_orders"`
	Items        []PaymentMethodBreakdownItemDTO `json:"items"`
}

// ============================================================================
// 2. REQUEST & QUERY PARAM DTOs
// ============================================================================

type OverviewStatsQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

type RevenueChartQuery struct {
	Period   string `form:"period" binding:"omitempty,oneof=daily monthly"` // daily | monthly (default: daily)
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`         // default: 30
	Months   int32  `form:"months" binding:"omitempty,min=1,max=36"`        // default: 12
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

type RecentOrdersQuery struct {
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=50"` // default: 5
}

type TopProductsQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"` // default: 30
	Limit    int32  `form:"limit" binding:"omitempty,min=1,max=50"` // default: 5
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

type OrderStatusDistributionQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`            // default: 30
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"` // YYYY-MM-DD
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`   // YYYY-MM-DD
}

type LowStockProductsQuery struct {
	Threshold int32 `form:"threshold" binding:"omitempty,min=0,max=100"` // default: 5
	Limit     int32 `form:"limit" binding:"omitempty,min=1,max=50"`      // default: 10
}

type PaymentMethodBreakdownQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`            // default: 30
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"` // YYYY-MM-DD
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`   // YYYY-MM-DD
}

type CartFunnelQuery struct {
	Days     int    `form:"days"`
	FromDate string `form:"from_date"`
	ToDate   string `form:"to_date"`
}

type CartFunnelDTO struct {
	TotalCarts           int64   `json:"total_carts"`
	OrdersCreated        int64   `json:"orders_created"`
	OrdersPaid           int64   `json:"orders_paid"`
	AbandonedCarts       int64   `json:"abandoned_carts"`
	AbandonmentRate      float64 `json:"abandonment_rate"`       // Percentage (0 - 100%)
	CartToOrderRate      float64 `json:"cart_to_order_rate"`     // Conversion (0 - 100%)
	OrderToPaidRate      float64 `json:"order_to_paid_rate"`     // Conversion (0 - 100%)
	PotentialLostRevenue float64 `json:"potential_lost_revenue"` // Currency value
}

// Type aliases for flexible naming conventions
type OrderStatusDistributionDTO = OrderStatusDistributionItemDTO
type OrderStatusDistributionParamsDTO = OrderStatusDistributionQuery
type LowStockProductsParamsDTO = LowStockProductsQuery
type PaymentMethodBreakdownDTO = PaymentMethodBreakdownItemDTO
type PaymentMethodBreakdownParamsDTO = PaymentMethodBreakdownQuery
