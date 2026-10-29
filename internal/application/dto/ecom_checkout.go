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

// ShippingRateGroup — ongkir untuk SATU toko.
//
// Keranjang boleh berisi barang dari dua toko, dan tiap toko dikirim dari
// alamat yang berbeda — jadi tarifnya dihitung sendiri-sendiri dan customer
// memilih kurir per toko. Ini yang dilakukan Tokopedia/Shopee, dan satu-satunya
// cara yang benar di sini karena satu `orders` cuma punya satu kurir, satu
// ongkir, dan satu resi.
type ShippingRateGroup struct {
	Location         string         `json:"location"` // "home" | "store"
	StoreName        string         `json:"store_name"`
	TotalWeightGrams int            `json:"total_weight_grams"`
	Subtotal         float64        `json:"subtotal"`
	Rates            []ShippingRate `json:"rates"`
	// Error — kalau toko INI gagal dihitung ongkirnya (mis. alamat jemputnya
	// belum diisi). Toko lain tetap bisa dipilih; mematikan seluruh halaman
	// gara-gara satu toko membuat customer tidak bisa belanja sama sekali.
	Error string `json:"error,omitempty"`
}

type ShippingRatesResponse struct {
	Address struct {
		Label         string `json:"label"`
		RecipientName string `json:"recipient_name"`
		City          string `json:"city"`
		Province      string `json:"province"`
	} `json:"address"`
	TotalWeightGrams int `json:"total_weight_grams"`
	// Groups — satu entri per toko yang punya barang di keranjang.
	//
	// Field datar `rates` yang lama SENGAJA dihapus, bukan dipertahankan
	// berisi grup pertama: bundel FE lama yang membacanya akan memakai ongkir
	// satu toko untuk seluruh keranjang — salah yang tidak kelihatan. Tanpa
	// field itu, FE lama gagal dengan jelas dan ketahuan seketika.
	Groups []ShippingRateGroup `json:"groups"`
}

// ─── Checkout ─────────────────────────────────────────────────────────

// CheckoutGroupRequest — pilihan pengiriman untuk SATU toko.
//
// Satu `orders` cuma muat satu kurir, satu ongkir, satu resi, dan satu titik
// jemput. Jadi keranjang berisi dua toko menghasilkan dua pesanan, masing-
// masing dengan kurir dan ongkirnya sendiri — persis seperti Tokopedia.
type CheckoutGroupRequest struct {
	Location string `json:"location" validate:"required,oneof=home store"`
	// Courier + Service = CODE Biteship (mis. "sicepat"+"reg"). Wajib supaya
	// downstream CreateBiteshipShipment berhasil call Biteship /v1/orders.
	// *Name = display friendly untuk UI. Opsional.
	ShippingCourier     string  `json:"shipping_courier" validate:"required"`
	ShippingCourierName string  `json:"shipping_courier_name,omitempty"`
	ShippingService     string  `json:"shipping_service" validate:"required"`
	ShippingServiceName string  `json:"shipping_service_name,omitempty"`
	ShippingCost        float64 `json:"shipping_cost" validate:"gte=0"`
	ShippingETD         string  `json:"shipping_etd"`
	// VoucherCode — voucher TOKO INI. Voucher milik toko lain ditolak, supaya
	// potongan satu toko tidak pernah ditanggung toko satunya.
	VoucherCode string `json:"voucher_code,omitempty"`
}

type CheckoutCreateRequest struct {
	AddressID string `json:"address_id" validate:"required,uuid"`
	// Groups — satu entri per toko yang ada barangnya di keranjang. Field
	// pengiriman yang dulu datar di sini SENGAJA dihapus, bukan dipertahankan
	// sebagai cadangan: payload lama yang lolos akan memakai satu kurir untuk
	// dua toko dan menghasilkan ongkir yang salah tanpa gejala. Sekarang
	// payload lama ditolak 400 dengan jelas.
	Groups []CheckoutGroupRequest `json:"groups" validate:"required,min=1,dive"`
	Notes  string                 `json:"notes,omitempty"`
	// PG channel yang customer pilih di checkout page (bca/bri/qris/ovo/dst).
	// TIDAK `required`: di mode transfer manual tidak ada kanal untuk dipilih,
	// jadi mewajibkannya membuat checkout manual selalu ditolak 400. Yang
	// memeriksa kewajibannya adalah usecase, yang tahu saklar PG menyala atau
	// tidak — validator struct tidak punya konteks itu.
	PaymentChannel         string `json:"payment_channel,omitempty"`
	PaymentChannelCategory string `json:"payment_channel_category,omitempty"`
	// Selected cart items — kalau kosong = pakai SEMUA cart item.
	// Kalau di-set = hanya item dengan id ini yang di-checkout (partial cart).
	// Item cart lain TIDAK di-hapus, tetap tersimpan untuk checkout berikutnya.
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

// CheckoutOrderSummary — satu pesanan hasil checkout (satu toko).
type CheckoutOrderSummary struct {
	OrderID         string  `json:"order_id"`
	Location        string  `json:"location"`
	StoreName       string  `json:"store_name"`
	Subtotal        float64 `json:"subtotal"`
	ShippingCost    float64 `json:"shipping_cost"`
	VoucherCode     string  `json:"voucher_code,omitempty"`
	VoucherDiscount float64 `json:"voucher_discount,omitempty"`
	Total           float64 `json:"total"`
}

type CheckoutCreateResponse struct {
	// CheckoutGroupID — pengikat semua pesanan dari satu kali checkout.
	// Satu pembayaran menutup seluruhnya.
	CheckoutGroupID string                 `json:"checkout_group_id"`
	Orders          []CheckoutOrderSummary `json:"orders"`
	// OrderID — pesanan PERTAMA. Dipertahankan supaya FE punya satu tujuan
	// jelas untuk diarahkan setelah checkout (dan jalur lama yang cuma butuh
	// satu id tetap jalan).
	OrderID string `json:"order_id"`
	// GrandTotal — yang harus ditransfer customer, mencakup semua toko.
	GrandTotal   float64 `json:"grand_total"`
	Subtotal     float64 `json:"subtotal"`
	ShippingCost float64 `json:"shipping_cost"`
	// Total — alias GrandTotal, untuk pemanggil yang sudah terbiasa.
	Total           float64 `json:"total"`
	VoucherDiscount float64 `json:"voucher_discount,omitempty"`
	// PG checkout URL — customer buka link ini untuk membayar.
	PaymentURL     string `json:"payment_url,omitempty"`
	PaymentChannel string `json:"payment_channel,omitempty"`
	// Kalau PG dimatikan/belum dikonfigurasi, mode="manual" — customer
	// diarahkan ke halaman pesanan dengan info transfer bank.
	PaymentMode string `json:"payment_mode"` // "pg" | "manual"
	EcomStatus  string `json:"ecom_status"`  // "pending_payment"
}

// ─── Customer order list ─────────────────────────────────────────────
type CustomerOrderListItem struct {
	ID            string  `json:"id"`
	Total         float64 `json:"total"`
	EcomStatus    string  `json:"ecom_status"`
	ItemCount     int     `json:"item_count"`
	FirstItem     string  `json:"first_item"` // preview: nama produk pertama
	CreatedAt     string  `json:"created_at"`
	PaymentPaidAt *string `json:"payment_paid_at,omitempty"`
	// Terisi kalau pesanan ini satu pembayaran dengan pesanan lain (keranjang
	// dua toko). Tanpa penandanya customer melihat dua baris untuk satu
	// transfer dan mengira ditagih dua kali.
	CheckoutGroupID string `json:"checkout_group_id,omitempty"`
	StoreName       string `json:"store_name,omitempty"`
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
	CreatedAt       string  `json:"created_at"`

	// Satu checkout bisa menghasilkan beberapa pesanan (keranjang dua toko),
	// dan customer membayarnya SEKALI. Tanpa keterangan ini admin melihat dua
	// tagihan untuk satu transfer dan berisiko mengira uangnya kurang —
	// atau, kalau dikonfirmasi dua kali, mengira uangnya masuk dua kali.
	CheckoutGroupID string              `json:"checkout_group_id,omitempty"`
	GroupOrders     []OrderGroupSibling `json:"group_orders,omitempty"`
	GroupTotal      float64             `json:"group_total,omitempty"`

	// Bukti transfer yang di-upload customer (URL di /storage). Kosong =
	// belum upload. Dipakai customer untuk melihat yang sudah dikirim, dan
	// admin untuk memeriksa sebelum mengonfirmasi pembayaran.
	PaymentProof string `json:"payment_proof,omitempty"`

	Items []CustomerOrderItemDetail `json:"items"`

	Shipping struct {
		// Courier + ServiceName = DISPLAY-friendly (mis. "SiCepat"+"Reguler").
		// FE render langsung. Code (sicepat/reg) tidak di-expose ke customer,
		// disimpan di DB untuk downstream Biteship API.
		Courier     string `json:"courier"`
		ServiceName string `json:"service_name"`
		ETD         string `json:"etd"`
		AWB         string `json:"awb,omitempty"`
		// TrackingURL = public Biteship link untuk customer klik "Lacak Paket".
		// Sudah include peta + history — hemat CS "resi sudah tapi belum update".
		TrackingURL     string `json:"tracking_url,omitempty"`
		BiteshipOrderID string `json:"biteship_order_id,omitempty"` // admin-visible; kalau tidak kosong = auto-resi via Biteship API
		Address         struct {
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

// OrderGroupSibling — pesanan lain dalam satu pembayaran yang sama.
type OrderGroupSibling struct {
	OrderID   string  `json:"order_id"`
	StoreName string  `json:"store_name"`
	Total     float64 `json:"total"`
	Status    string  `json:"status,omitempty"`
	IsSelf    bool    `json:"is_self"`
}

type CustomerOrderItemDetail struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Subtotal  float64 `json:"subtotal"`
	Image     string  `json:"image,omitempty"`
}
