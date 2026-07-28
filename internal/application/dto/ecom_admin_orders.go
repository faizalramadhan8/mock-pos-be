package dto

// EcomAdminOrderListItem — 1 row untuk table view "Pesanan Online" di admin
// panel. Info compact — admin klik row untuk lihat detail full.
type EcomAdminOrderListItem struct {
	ID            string  `json:"id"`
	Total         float64 `json:"total"`
	ShippingCost  float64 `json:"shipping_cost"`
	EcomStatus    string  `json:"ecom_status"`
	ItemCount     int     `json:"item_count"`
	Recipient     string  `json:"recipient"`
	Courier       string  `json:"courier,omitempty"`
	AWB           string  `json:"awb,omitempty"`
	CreatedAt     string  `json:"created_at"`
	PaymentMethod string  `json:"payment_method,omitempty"`
}

// EcomAdminOrderListResponse — list + cursor + count summary per status
// untuk render badge di tab filter FE.
type EcomAdminOrderListResponse struct {
	Items          []EcomAdminOrderListItem `json:"items"`
	NextCursor     string                   `json:"next_cursor,omitempty"`
	CountsByStatus map[string]int           `json:"counts_by_status"`
}

// AdminUpdateStatusRequest — body untuk PATCH ubah status manual.
type AdminUpdateStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=paid processing shipped completed cancelled"`
}

// AdminSetShippingRequest — body untuk input resi (auto-transition ke shipped).
type AdminSetShippingRequest struct {
	AWB     string `json:"awb" validate:"required,min=3,max=100"`
	Courier string `json:"courier,omitempty"`
	Service string `json:"service,omitempty"`
}
