package entity

import (
	"time"
	"gorm.io/gorm"
)

type Order struct {
	ID                 string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Items              []OrderItem    `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	Payments           []OrderPayment `gorm:"foreignKey:OrderID" json:"payments,omitempty"`
	Subtotal           float64        `gorm:"type:decimal(15,2);not null;default:0" json:"subtotal"`
	PPNRate            float64        `gorm:"column:ppn_rate;type:decimal(5,2);not null;default:11" json:"ppn_rate"`
	PPN                float64        `gorm:"type:decimal(15,2);not null;default:0" json:"ppn"`
	Total              float64        `gorm:"type:decimal(15,2);not null;default:0" json:"total"`
	Payment            string         `gorm:"type:varchar(20);not null;default:'cash'" json:"payment"`
	// Payment edit audit inline (migration 000045). Kalau NULL berarti order
	// tidak pernah di-edit metode bayar-nya. Detail action tetap ke audit_log.
	PaymentsEditedAt     *time.Time `gorm:"column:payments_edited_at;type:datetime;null" json:"payments_edited_at,omitempty"`
	PaymentsEditedBy     *string    `gorm:"column:payments_edited_by;type:varchar(36);null" json:"payments_edited_by,omitempty"`
	PaymentsEditedReason *string    `gorm:"column:payments_edited_reason;type:varchar(500);null" json:"payments_edited_reason,omitempty"`
	Status             string         `gorm:"type:varchar(20);not null;default:'completed'" json:"status"`
	Customer           string         `gorm:"type:varchar(200);null" json:"customer,omitempty"`
	CustomerPhone      string         `gorm:"type:varchar(20);null" json:"customer_phone,omitempty"`
	MemberID           *string        `gorm:"type:varchar(36);null;index" json:"member_id,omitempty"`
	Member             *Member        `gorm:"foreignKey:MemberID" json:"member,omitempty"`
	PaymentProof       string         `gorm:"type:text;null" json:"payment_proof,omitempty"`
	OrderDiscountType  string         `gorm:"type:varchar(20);null" json:"order_discount_type,omitempty"`
	OrderDiscountValue float64        `gorm:"type:decimal(15,2);null;default:0" json:"order_discount_value"`
	OrderDiscount      float64        `gorm:"type:decimal(15,2);null;default:0" json:"order_discount"`
	CreatedBy          string         `gorm:"type:varchar(36);not null" json:"created_by"`
	// Ecom fields (migration 000051, Bu Santi 24 Jul 2026). Semua NULL untuk
	// order POS existing.
	OrderSource             string     `gorm:"column:order_source;type:varchar(10);not null;default:pos" json:"order_source"`
	EcomUserID              *string    `gorm:"column:ecom_user_id;type:varchar(36);null" json:"ecom_user_id,omitempty"`
	ShippingAddressSnapshot *string    `gorm:"column:shipping_address_snapshot;type:json;null" json:"shipping_address_snapshot,omitempty"`
	// ShippingCourier = CODE Biteship (mis. "sicepat", "jnt", "jne"). Sejak
	// migration 000061 (28 Jul 2026) — sebelumnya isi display name yang bikin
	// CreateBiteshipShipment reject. Display name pindah ke ShippingCourierName.
	ShippingCourier         *string    `gorm:"column:shipping_courier;type:varchar(50);null" json:"shipping_courier,omitempty"`
	ShippingCourierName     *string    `gorm:"column:shipping_courier_name;type:varchar(100);null" json:"shipping_courier_name,omitempty"`
	// ShippingService = CODE Biteship (mis. "reg", "yes"). Display di *_name.
	ShippingService         *string    `gorm:"column:shipping_service;type:varchar(50);null" json:"shipping_service,omitempty"`
	ShippingServiceName     *string    `gorm:"column:shipping_service_name;type:varchar(100);null" json:"shipping_service_name,omitempty"`
	ShippingCost            float64    `gorm:"column:shipping_cost;type:decimal(15,2);null;default:0" json:"shipping_cost,omitempty"`
	VoucherCode             *string    `gorm:"column:voucher_code;type:varchar(50);null" json:"voucher_code,omitempty"`
	VoucherDiscount         float64    `gorm:"column:voucher_discount;type:decimal(15,2);not null;default:0" json:"voucher_discount,omitempty"`
	ShippingETD             *string    `gorm:"column:shipping_etd;type:varchar(50);null" json:"shipping_etd,omitempty"`
	ShippingAWB             *string    `gorm:"column:shipping_awb;type:varchar(100);null" json:"shipping_awb,omitempty"`
	// Public tracking URL dari Biteship (courier.link di response). Customer
	// klik langsung buka page tracking kurir tanpa copy AWB manual.
	ShippingTrackingURL     *string    `gorm:"column:shipping_tracking_url;type:varchar(500);null" json:"shipping_tracking_url,omitempty"`
	// Biteship internal tracking_id — dipakai GET /v1/trackings/:id untuk
	// manual sync kalau webhook missed.
	ShippingTrackingID      *string    `gorm:"column:shipping_tracking_id;type:varchar(64);null" json:"shipping_tracking_id,omitempty"`
	BiteshipOrderID         *string    `gorm:"column:biteship_order_id;type:varchar(64);null" json:"biteship_order_id,omitempty"`
	EcomStatus              *string    `gorm:"column:ecom_status;type:varchar(30);null" json:"ecom_status,omitempty"`
	// Waktu Biteship (atau admin manual) tandai "sudah sampai kurir".
	// Bukan waktu customer konfirmasi terima — itu tetap pakai UpdatedAt saat
	// transisi delivered → completed. Dipakai cron auto-complete 7 hari.
	EcomDeliveredAt         *time.Time `gorm:"column:ecom_delivered_at;type:datetime;null" json:"ecom_delivered_at,omitempty"`
	PaymentSnapToken        *string    `gorm:"column:payment_snap_token;type:varchar(100);null" json:"payment_snap_token,omitempty"`
	// Payment Gateway DOKU (28 Jul 2026). PaymentSnapToken di-deprecate untuk
	// order ecom baru — semua checkout ke PGWrapper via internal/infrastructure/pg.
	// PaymentURL = DOKU checkout link yang customer buka. Channel/Category
	// snapshot pilihan customer (bca/qris/dst).
	PaymentURL             *string    `gorm:"column:payment_url;type:varchar(500);null" json:"payment_url,omitempty"`
	PaymentChannel         *string    `gorm:"column:payment_channel;type:varchar(20);null" json:"payment_channel,omitempty"`
	PaymentChannelCategory *string    `gorm:"column:payment_channel_category;type:varchar(30);null" json:"payment_channel_category,omitempty"`
	PaymentReference        *string    `gorm:"column:payment_reference;type:varchar(100);null" json:"payment_reference,omitempty"`
	PaymentPaidAt           *time.Time `gorm:"column:payment_paid_at;type:datetime;null" json:"payment_paid_at,omitempty"`
	PaymentExpiredAt        *time.Time `gorm:"column:payment_expired_at;type:datetime;null" json:"payment_expired_at,omitempty"`
	CreatedAt          time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt          time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Order) TableName() string { return "orders" }

type OrderItem struct {
	ID      string `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	OrderID string `gorm:"type:varchar(36);not null" json:"order_id"`
	// ProductID: empty string ("") kalau row ini redeem dari redeemable_items
	// table. Cek `RedeemableItemID != nil` untuk detect redeem row.
	ProductID string `gorm:"type:varchar(36);null" json:"product_id"`
	// RedeemableItemID nullable FK ke redeemable_items. ON DELETE SET NULL
	// supaya delete item tebus tidak hilang history order. Lihat migration 000041.
	RedeemableItemID *string `gorm:"column:redeemable_item_id;type:varchar(36);null" json:"redeemable_item_id,omitempty"`
	Name           string    `gorm:"type:varchar(200);not null" json:"name"`
	Quantity       int       `gorm:"type:int;not null;default:1" json:"quantity"`
	UnitType       string    `gorm:"type:varchar(20);not null;default:'individual'" json:"unit_type"`
	UnitPrice      float64   `gorm:"type:decimal(15,2);not null;default:0" json:"unit_price"`
	PurchasePrice  float64   `gorm:"type:decimal(15,2);null" json:"purchase_price,omitempty"`
	RegularPrice   *float64  `gorm:"type:decimal(15,2);null" json:"regular_price,omitempty"`
	DiscountType   string    `gorm:"type:varchar(20);null" json:"discount_type,omitempty"`
	DiscountValue  float64   `gorm:"type:decimal(15,2);null;default:0" json:"discount_value"`
	DiscountAmount float64   `gorm:"type:decimal(15,2);null;default:0" json:"discount_amount"`
	// RedeemedWithPoints: true kalau item ini dibayar pakai member.points
	// (tebus barang). Harga item tidak masuk hitungan cash actual untuk
	// earn poin baru — cegah loop (tebus pakai poin lalu dapat poin lagi).
	RedeemedWithPoints bool `gorm:"column:redeemed_with_points;type:tinyint(1);not null;default:0" json:"redeemed_with_points"`
	// PriceSource: tag sumber harga saat sale time untuk audit.
	// Values: 'regular' | 'member_price' | 'tier_all' | 'tier_member'.
	// Default 'regular'. Lihat migration 000037.
	PriceSource string `gorm:"column:price_source;type:varchar(20);not null;default:'regular'" json:"price_source"`
	// TierID: nullable FK ke product_price_tiers kalau harga dari tier match.
	// ON DELETE SET NULL — delete tier tidak hilang history order.
	TierID *string `gorm:"column:tier_id;type:varchar(36);null" json:"tier_id,omitempty"`
	// PaketCount + ExtraCount: snapshot pecahan paket dari paket logic.
	// paket_count = floor(qty_satuan / tier.min_qty), extra = sisa.
	// Disnapshot supaya laporan bundling tetap akurat walaupun tier dihapus.
	// Lihat migration 000039.
	PaketCount int       `gorm:"column:paket_count;type:int;not null;default:0" json:"paket_count"`
	ExtraCount int       `gorm:"column:extra_count;type:int;not null;default:0" json:"extra_count"`
	CreatedAt  time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (OrderItem) TableName() string { return "order_items" }

// OrderPayment is one leg of a (possibly split) payment for an order.
// A single order may have multiple rows — e.g. cash 50.000 + qris 30.000
// for a total of 80.000. The sum of all payments must be >= order.total;
// any excess is the customer's change and is not stored here (it's derived
// at the checkout UI).
type OrderPayment struct {
	ID        string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	OrderID   string    `gorm:"type:varchar(36);not null;index" json:"order_id"`
	Method    string    `gorm:"type:varchar(20);not null" json:"method"`
	Amount    float64   `gorm:"type:decimal(15,2);not null;default:0" json:"amount"`
	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (OrderPayment) TableName() string { return "order_payments" }
