package dto

// Sprint 4 Chunk 1 (30 Jul 2026) — Customer management admin panel.
// Data source: users WHERE role='user'. Metrik order (total, spent, last)
// dihitung via LEFT JOIN ke ecom orders.

type EcomAdminCustomerListItem struct {
	ID            string   `json:"id"`
	FullName      string   `json:"full_name"`
	Email         string   `json:"email"`
	Phone         string   `json:"phone"`
	IsActive      bool     `json:"is_active"`
	OrderCount    int      `json:"order_count"`     // hanya ecom orders (bukan POS)
	TotalSpent    float64  `json:"total_spent"`     // sum(orders.total) status=completed
	LastOrderDate *string  `json:"last_order_date"` // ISO 8601, null kalau belum pernah order
	CreatedAt     string   `json:"created_at"`
}

type EcomAdminCustomerListResponse struct {
	Items      []EcomAdminCustomerListItem `json:"items"`
	NextCursor string                      `json:"next_cursor"`
	Total      int64                       `json:"total"`
}

// Detail — buka detail per customer di modal. Include daftar order recent
// (limit 20) + statistik agregat.
type EcomAdminCustomerDetail struct {
	EcomAdminCustomerListItem
	// Recent orders — untuk drill-down. Limit 20 supaya modal cepat.
	RecentOrders []EcomAdminCustomerOrderRow `json:"recent_orders"`
	// Address book count (bukan detail) — signal kalau customer verified alamat.
	AddressCount int `json:"address_count"`
	// Aggregate lain
	AvgOrderValue float64 `json:"avg_order_value"`
	CompletedCount int    `json:"completed_count"`
	PendingCount   int    `json:"pending_count"`
	CancelledCount int    `json:"cancelled_count"`
}

type EcomAdminCustomerOrderRow struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"created_at"`
	Status    string  `json:"status"`
	Total     float64 `json:"total"`
	ItemCount int     `json:"item_count"`
}
