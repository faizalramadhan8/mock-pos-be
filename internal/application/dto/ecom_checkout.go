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
	// PG channel yang customer pilih di checkout page (bca/bri/qris/ovo/dst).
	// Wajib (validate required) — tanpa channel PG tidak bisa create payment.
	// Category (virtual-account/qris/e-wallet/credit-card) opsional untuk audit.
	PaymentChannel         string `json:"payment_channel" validate:"required"`
	PaymentChannelCategory string `json:"payment_channel_category,omitempty"`
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
	// PG DOKU checkout URL — customer buka link ini untuk lakukan pembayaran
	// (VA number tampil, QR code muncul, e-wallet redirect, dsb).
	PaymentURL     string `json:"payment_url,omitempty"`
	PaymentChannel string `json:"payment_channel,omitempty"` // echo pilihan customer
	// Kalau PG belum configured, mode="manual" — customer diarahkan ke
	// halaman order dengan info bank transfer manual.
	PaymentMode string `json:"payment_mode"` // "pg" | "manual"
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
	// Waktu kurir tandai sampai. FE tampil "Sampai N hari lalu — konfirmasi
	// ya!" reminder + gate tombol "Konfirmasi Diterima". NULL untuk order
	// yang belum sampai (pending_payment/paid/processing/shipped).
	EcomDeliveredAt *string `json:"ecom_delivered_at,omitempty"`
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
		Mode string `json:"mode"` // "pg" | "manual"
		// PG DOKU (28 Jul 2026 — ganti Midtrans). PaymentURL = DOKU checkout link.
		// Channel/Category snapshot pilihan customer untuk display badge di UI.
		PaymentURL      string  `json:"payment_url,omitempty"`
		Channel         string  `json:"channel,omitempty"`
		ChannelCategory string  `json:"channel_category,omitempty"`
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
