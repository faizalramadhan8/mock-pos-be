package dto

// CartItemResponse — 1 row cart dengan info produk buat display.
type CartItemResponse struct {
	ID          string   `json:"id"`
	ProductID   string   `json:"product_id"`
	Name        string   `json:"name"`
	NameID      string   `json:"name_id"`
	SKU         string   `json:"sku"`
	Image       string   `json:"image,omitempty"`
	Quantity    int      `json:"quantity"`
	Price       float64  `json:"price"`         // ecom_price fallback selling_price
	MemberPrice *float64 `json:"member_price,omitempty"`
	Stock       int      `json:"stock"`         // stock_ecom saat ini (untuk validasi)
	MinOrder    int      `json:"min_order"`
	WeightGrams *int     `json:"weight_grams,omitempty"`
	Subtotal    float64  `json:"subtotal"`      // quantity × price
	// Kalau produk sudah tidak tersedia (dihapus / disabled / stok 0), flag
	// untuk UI display warning + prevent checkout.
	Unavailable bool   `json:"unavailable,omitempty"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// CartResponse — full cart dengan total.
type CartResponse struct {
	Items         []CartItemResponse `json:"items"`
	ItemCount     int                `json:"item_count"`      // total unique products
	TotalQty      int                `json:"total_qty"`       // sum of quantities
	Subtotal      float64            `json:"subtotal"`
	TotalWeight   int                `json:"total_weight_grams"`
	HasUnavailable bool              `json:"has_unavailable"` // any item unavailable
}

// CartAddRequest — add ke cart. Kalau product sudah ada, increment quantity.
type CartAddRequest struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Quantity  int    `json:"quantity" validate:"required,min=1,max=1000"`
}

// CartUpdateRequest — set quantity absolut. Kalau 0, delete item (client
// convenience). Backend enforce min_order dan stock check.
type CartUpdateRequest struct {
	Quantity int `json:"quantity" validate:"min=0,max=1000"`
}
