package dto

// EcomAdminDashboard — response GET /ecom/admin/dashboard.
type EcomAdminDashboard struct {
	TodayRevenue     float64             `json:"today_revenue"`
	TodayOrders      int                 `json:"today_orders"`
	MonthRevenue     float64             `json:"month_revenue"`
	MonthOrders      int                 `json:"month_orders"`
	// StatusCounts — count per ecom_status, misal {"paid":3, "shipped":5, ...}
	StatusCounts     map[string]int      `json:"status_counts"`
	TopProductsMonth []EcomDashTopProduct `json:"top_products_month"`
	TotalCustomers   int                 `json:"total_customers"`
	ComplaintsPending int                `json:"complaints_pending"`
	LowStockCount    int                 `json:"low_stock_count"`
}

type EcomDashTopProduct struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	QtySold   int     `json:"qty_sold"`
	Revenue   float64 `json:"revenue"`
}

type EcomLowStockItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	StockEcom int    `json:"stock_ecom"`
}
