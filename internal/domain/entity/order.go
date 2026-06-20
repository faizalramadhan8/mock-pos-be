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
