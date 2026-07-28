package dto

// ─── Shipping ─────────────────────────────────────────────────────────
type ShippingRateRequest struct {
	AddressID string `json:"address_id" validate:"required,uuid"`
	// Kalau non-empty, ongkir dihitung berdasar berat item ini saja (partial
	// cart / buy-now). Empty = pakai seluruh cart. BuyNowItems override
	// SelectedItemIDs kalau dua-duanya diisi.
	SelectedItemIDs []string     `json:"selected_item_ids,omitempty"`
	BuyNowItems     []BuyNowItem `json:"buy_now_items,omitempty"`
}

type ShippingRate struct {
	Courier     string  `json:"courier"`
	CourierName string  `json:"courier_name"`
	Service     string  `json:"service"`
	ServiceName string  `json:"service_name"`
	Cost        float64 `json:"cost"`
	ETD         string  `json:"etd"`
}

type ShippingRatesResponse struct {
	Address struct {
		Label         string `json:"label"`
		RecipientName string `json:"recipient_name"`
		City          string `json:"city"`
		Province      string `json:"province"`
	} `json:"address"`
	TotalWeightGrams int            `json:"total_weight_grams"`
	Rates            []ShippingRate `json:"rates"`
}

// ─── Checkout ─────────────────────────────────────────────────────────
type CheckoutCreateRequest struct {
	AddressID       string  `json:"address_id" validate:"required,uuid"`
	ShippingCourier string  `json:"shipping_courier" validate:"required"`
	ShippingService string  `json:"shipping_service" validate:"required"`
	ShippingCost    float64 `json:"shipping_cost" validate:"gte=0"`
	ShippingETD     string  `json:"shipping_etd"`
	Notes           string  `json:"notes,omitempty"`
	VoucherCode     string  `json:"voucher_code,omitempty"` // opsional
	// Selected cart items — kalau kosong = pakai SEMUA cart item (backward compat).
	// Kalau di-set = hanya item dengan id ini yang di-checkout (partial cart).
	// Item cart lain TIDAK di-hapus, tetap tersimpan di cart untuk checkout berikutnya.
	SelectedItemIDs []string `json:"selected_item_ids,omitempty"`
	// Buy-now mode — bypass cart entirely. Kalau di-set, checkout langsung
	// dari list ini (bukan dari cart DB). Cart user tidak disentuh.
	BuyNowItems []BuyNowItem `json:"buy_now_items,omitempty"`
}

// BuyNowItem — payload direct-checkout dari tombol "Beli Sekarang" di PDP.
type BuyNowItem struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Quantity  int    `json:"quantity" validate:"required,min=1"`
}

type CheckoutCreateResponse struct {
	OrderID         string  `json:"order_id"`
	Subtotal        float64 `json:"subtotal"`
	ShippingCost    float64 `json:"shipping_cost"`
	VoucherCode     string  `json:"voucher_code,omitempty"`
	VoucherDiscount float64 `json:"voucher_discount,omitempty"`
	Total           float64 `json:"total"`
	SnapToken       string  `json:"snap_token,omitempty"`
	SnapRedirectURL string  `json:"snap_redirect_url,omitempty"`
	// Kalau Midtrans belum configured, mode="manual" — customer diarahkan ke
	// halaman order dengan info bank transfer manual.
	PaymentMode string `json:"payment_mode"` // "midtrans" | "manual"
	EcomStatus  string `json:"ecom_status"`  // "pending_payment"
}

// ─── Customer order list ─────────────────────────────────────────────
type CustomerOrderListItem struct {
	ID          string  `json:"id"`
	Total       float64 `json:"total"`
	EcomStatus  string  `json:"ecom_status"`
	ItemCount   int     `json:"item_count"`
	FirstItem   string  `json:"first_item"` // preview: nama produk pertama
	CreatedAt   string  `json:"created_at"`
	PaymentPaidAt *string `json:"payment_paid_at,omitempty"`
}

type CustomerOrderDetail struct {
	ID           string  `json:"id"`
	Subtotal     float64 `json:"subtotal"`
	ShippingCost float64 `json:"shipping_cost"`
	Total        float64 `json:"total"`
	EcomStatus   string  `json:"ecom_status"`
	CreatedAt    string  `json:"created_at"`

	Items []CustomerOrderItemDetail `json:"items"`

	Shipping struct {
		Courier     string `json:"courier"`
		ServiceName string `json:"service_name"`
		ETD         string `json:"etd"`
		AWB         string `json:"awb,omitempty"`
		BiteshipOrderID string `json:"biteship_order_id,omitempty"` // admin-visible; kalau tidak kosong = auto-resi via Biteship API
		Address     struct {
			Label          string `json:"label"`
			RecipientName  string `json:"recipient_name"`
			RecipientPhone string `json:"recipient_phone"`
			StreetAddress  string `json:"street_address"`
			Subdistrict    string `json:"subdistrict"`
			District       string `json:"district"`
			City           string `json:"city"`
			Province       string `json:"province"`
			Zipcode        string `json:"zipcode"`
			Notes          string `json:"notes,omitempty"`
		} `json:"address"`
	} `json:"shipping"`

	Payment struct {
		Mode            string  `json:"mode"`               // "midtrans" | "manual"
		SnapToken       string  `json:"snap_token,omitempty"`
		SnapRedirectURL string  `json:"snap_redirect_url,omitempty"` // full URL to Midtrans hosted checkout
		Reference       string  `json:"reference,omitempty"`
		PaidAt          *string `json:"paid_at,omitempty"`
		ExpiredAt       *string `json:"expired_at,omitempty"`
	} `json:"payment"`
}

type CustomerOrderItemDetail struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Subtotal  float64 `json:"subtotal"`
	Image     string  `json:"image,omitempty"`
}
