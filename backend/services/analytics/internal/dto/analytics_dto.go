package dto

// Response DTOs and query params for the admin analytics endpoints. Field
// names and JSON tags are the dashboard contract; keep them stable.

type OverviewStatsDTO struct {
	TotalRevenue   float64 `json:"total_revenue"`
	TotalOrders    int64   `json:"total_orders"`
	NewCustomers   int64   `json:"new_customers"`
	PendingOrders  int64   `json:"pending_orders"`
	RevenueGrowth  float64 `json:"revenue_growth"`
	OrderGrowth    float64 `json:"order_growth"`
	CustomerGrowth float64 `json:"customer_growth"`
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

type OrderStatusDistributionItemDTO struct {
	Status      int32   `json:"status"`
	StatusName  string  `json:"status_name"`
	Count       int64   `json:"count"`
	TotalAmount float64 `json:"total_amount"`
	Percentage  float64 `json:"percentage"`
}

type OrderStatusDistributionResponse struct {
	TotalOrders int64                            `json:"total_orders"`
	Items       []OrderStatusDistributionItemDTO `json:"items"`
}

type LowStockProductDTO struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"`
	ImageURL     *string `json:"image_url,omitempty"`
	CategoryName string  `json:"category_name"`
	AlertLevel   string  `json:"alert_level"` // "critical" (stock <= 0) or "warning"
}

type LowStockProductResponse struct {
	Threshold int32                `json:"threshold"`
	TotalLow  int64                `json:"total_low"`
	Items     []LowStockProductDTO `json:"items"`
}

type PaymentMethodBreakdownItemDTO struct {
	PaymentMethod int32   `json:"payment_method"`
	MethodName    string  `json:"method_name"`
	OrderCount    int64   `json:"order_count"`
	NetRevenue    float64 `json:"net_revenue"`
	Percentage    float64 `json:"percentage"`
}

type PaymentMethodBreakdownResponse struct {
	TotalRevenue float64                         `json:"total_revenue"`
	TotalOrders  int64                           `json:"total_orders"`
	Items        []PaymentMethodBreakdownItemDTO `json:"items"`
}

// Query params.

type OverviewStatsQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

type RevenueChartQuery struct {
	Period   string `form:"period" binding:"omitempty,oneof=daily monthly"`
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`
	Months   int32  `form:"months" binding:"omitempty,min=1,max=36"`
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

type RecentOrdersQuery struct {
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=50"`
}

type TopProductsQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`
	Limit    int32  `form:"limit" binding:"omitempty,min=1,max=50"`
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

type OrderStatusDistributionQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

type LowStockProductsQuery struct {
	Threshold int32 `form:"threshold" binding:"omitempty,min=0,max=100"`
	Limit     int32 `form:"limit" binding:"omitempty,min=1,max=50"`
}

type PaymentMethodBreakdownQuery struct {
	Days     int32  `form:"days" binding:"omitempty,min=1,max=365"`
	FromDate string `form:"from_date" binding:"omitempty,datetime=2006-01-02"`
	ToDate   string `form:"to_date" binding:"omitempty,datetime=2006-01-02"`
}

func MapOrderStatusToString(status int32) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Confirmed"
	case 3:
		return "Shipped"
	case 4:
		return "Delivered"
	case 5:
		return "Cancelled"
	case 6:
		return "Returned"
	default:
		return "Unknown"
	}
}

func MapPaymentMethodToString(method int32) string {
	switch method {
	case 1:
		return "COD"
	case 2:
		return "Bank Transfer"
	case 3:
		return "E-Wallet"
	default:
		return "Unknown"
	}
}