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
	UUID            string  `json:"uuid"`
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	Price           float64 `json:"price"`
	ImageURL        *string `json:"image_url,omitempty"`
	CategoryName    string  `json:"category_name"`
	TotalUnitsSold  int64   `json:"total_units_sold"`
	TotalRevenue    float64 `json:"total_revenue"`
}

// ============================================================================
// 2. REQUEST & QUERY PARAM DTOs
// ============================================================================

type RevenueChartQuery struct {
	Period string `form:"period" binding:"omitempty,oneof=daily monthly"` // daily | monthly (default: daily)
	Days   int32  `form:"days" binding:"omitempty,min=1,max=90"`          // default: 30
	Months int32  `form:"months" binding:"omitempty,min=1,max=36"`        // default: 12
}

type RecentOrdersQuery struct {
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=50"` // default: 5
}

type TopProductsQuery struct {
	Days  int32 `form:"days" binding:"omitempty,min=1,max=90"`   // default: 30
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=50"` // default: 5
}
