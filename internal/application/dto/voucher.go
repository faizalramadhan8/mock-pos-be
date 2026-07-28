package dto

// VoucherValidateRequest — customer input di checkout. Subtotal dihitung
// BE sendiri dari cart (jangan trust FE subtotal — cegah exploit).
type VoucherValidateRequest struct {
	Code string `json:"code" validate:"required,min=2,max=50"`
}

// VoucherValidateResponse — FE tampilkan preview discount + description.
type VoucherValidateResponse struct {
	Code        string  `json:"code"`
	Description string  `json:"description,omitempty"`
	Type        string  `json:"type"`      // 'percent' | 'fixed'
	Value       float64 `json:"value"`     // raw config (10 = 10% atau 10000 = Rp 10rb)
	Discount    float64 `json:"discount"`  // computed amount setelah cap/clamp
}

// VoucherCreateRequest — admin CRUD payload (share Create + Update).
type VoucherCreateRequest struct {
	Code        string   `json:"code" validate:"required,min=2,max=50"`
	Description string   `json:"description" validate:"omitempty,max=200"`
	Type        string   `json:"type" validate:"required,oneof=percent fixed"`
	Value       float64  `json:"value" validate:"required,gt=0"`
	MinSubtotal float64  `json:"min_subtotal" validate:"omitempty,gte=0"`
	MaxDiscount *float64 `json:"max_discount,omitempty"`
	UsageLimit  int      `json:"usage_limit" validate:"omitempty,gte=0"`
	StartsAt    string   `json:"starts_at"`  // ISO/YYYY-MM-DD, empty = no start
	ExpiresAt   string   `json:"expires_at"` // ISO/YYYY-MM-DD, empty = no expiry
	IsActive    *bool    `json:"is_active,omitempty"`
}
