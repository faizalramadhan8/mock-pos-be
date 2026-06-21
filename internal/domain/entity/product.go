package entity

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Category struct {
	ID        string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Name      string         `gorm:"type:varchar(100);not null" json:"name"`
	NameID    string         `gorm:"column:name_id;type:varchar(100);not null" json:"name_id"`
	Icon      string         `gorm:"type:varchar(50);null" json:"icon,omitempty"`
	Color     string         `gorm:"type:varchar(20);null" json:"color,omitempty"`
	CreatedAt time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Category) TableName() string { return "categories" }

type Product struct {
	ID            string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	SKU           string         `gorm:"type:varchar(50);not null;uniqueIndex" json:"sku"`
	Barcode       string         `gorm:"type:varchar(50);null;index" json:"barcode,omitempty"`
	Name          string         `gorm:"type:varchar(200);not null" json:"name"`
	NameID        string         `gorm:"column:name_id;type:varchar(200);not null" json:"name_id"`
	CategoryID    string         `gorm:"type:varchar(36);not null" json:"category_id"`
	Category      *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	SupplierID    *string        `gorm:"type:varchar(36);null" json:"supplier_id,omitempty"`
	Supplier      *Supplier      `gorm:"foreignKey:SupplierID" json:"supplier,omitempty"`
	PurchasePrice float64        `gorm:"type:decimal(15,2);not null;default:0" json:"purchase_price"`
	SellingPrice  float64        `gorm:"type:decimal(15,2);not null;default:0" json:"selling_price"`
	MemberPrice   *float64       `gorm:"type:decimal(15,2);null" json:"member_price,omitempty"`
	QtyPerBox     int            `gorm:"type:int;not null;default:1" json:"qty_per_box"`
	Stock         int            `gorm:"type:int;not null;default:0" json:"stock"`
	Unit          string         `gorm:"type:varchar(20);not null;default:'pcs'" json:"unit"`
	Image         string         `gorm:"type:text;null" json:"image,omitempty"`
	MinStock      int            `gorm:"type:int;not null;default:0" json:"min_stock"`
	IsActive      bool           `gorm:"type:tinyint(1);not null;default:1" json:"is_active"`
	// IsRedeemable: TRUE = produk masuk katalog tebus poin. Admin tandai
	// via halaman "Katalog Tebus Poin" di Stok. Default FALSE — semua
	// produk existing tidak eligible sampai di-curate.
	IsRedeemable  bool           `gorm:"column:is_redeemable;type:tinyint(1);not null;default:0" json:"is_redeemable"`
	CreatedAt     time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt     time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Product) TableName() string { return "products" }

// ProductPriceTier — tiered pricing untuk member. Admin set "beli ≥ min_qty
// satuan dapat harga X" yang berlaku khusus member (umum atau spesifik).
// Walk-in customer non-member selalu pakai selling_price normal.
//
// min_qty SELALU dalam satuan. Kalau cart unit_type=box, BE/FE convert dulu
// (qty × qty_per_box) sebelum compare. price = harga per satuan juga (bukan
// per dus) — konsisten dengan member_price/selling_price baseline.
type ProductPriceTier struct {
	ID         string     `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	ProductID  string     `gorm:"column:product_id;type:varchar(36);not null" json:"product_id"`
	MinQty     int        `gorm:"column:min_qty;type:int;not null" json:"min_qty"`
	Price      float64    `gorm:"type:decimal(15,2);not null" json:"price"`
	TargetType string     `gorm:"column:target_type;type:varchar(20);not null" json:"target_type"` // 'all_customers' | 'member_specific'
	Note       string     `gorm:"type:varchar(200);null" json:"note,omitempty"`
	// ExpiresAt: tier auto-balik ke harga normal setelah ini. NULL = tidak
	// terbatas. Per request Bu Santi 21 Jun 2026 — durasi opt-in (1/3/6/12/30
	// hari) untuk promo terbatas. POS compute skip tier dengan expires_at <
	// NOW. Lihat migration 000042.
	ExpiresAt *time.Time `gorm:"column:expires_at;type:datetime;null;index" json:"expires_at,omitempty"`
	CreatedAt time.Time  `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time  `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`

	// Members: kalau target_type='member_specific', list whitelist member.
	// Preloaded via gorm many2many. Empty untuk target_type='all_customers'.
	Members []Member `gorm:"many2many:product_price_tier_members;foreignKey:ID;joinForeignKey:tier_id;References:ID;joinReferences:member_id" json:"members,omitempty"`
}

func (ProductPriceTier) TableName() string { return "product_price_tiers" }

type Supplier struct {
	ID        string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Name      string         `gorm:"type:varchar(200);not null" json:"name"`
	Phone     string         `gorm:"type:varchar(20);null" json:"phone,omitempty"`
	Email     string         `gorm:"type:varchar(100);null" json:"email,omitempty"`
	Address   string         `gorm:"type:text;null" json:"address,omitempty"`
	CreatedAt time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Supplier) TableName() string { return "suppliers" }

// ProductPriceHistory keeps an append-only log of every price change for a
// product. Reports that need historical accuracy (profit, sales by tier) can
// look up the price active at a given timestamp via:
//   start_date <= t AND (end_date IS NULL OR end_date > t)
// price_type values: "regular" | "member" | "purchase".
// status values:     "active"  | "inactive".
type ProductPriceHistory struct {
	ID         string     `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	ProductID  string     `gorm:"type:varchar(36);not null;index" json:"product_id"`
	PriceType  string     `gorm:"type:varchar(20);not null" json:"price_type"`
	Price      float64    `gorm:"type:decimal(15,2);not null;default:0" json:"price"`
	Status     string     `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	StartDate  time.Time  `gorm:"not null;default:current_timestamp()" json:"start_date"`
	EndDate    *time.Time `gorm:"null" json:"end_date,omitempty"`
	ChangedBy  *string    `gorm:"type:varchar(36);null" json:"changed_by,omitempty"`
	Note       string     `gorm:"type:varchar(255);null" json:"note,omitempty"`
	CreatedAt  time.Time  `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (ProductPriceHistory) TableName() string { return "product_price_history" }

// ProductPriceTierHistory: audit append-only untuk CRUD product_price_tiers.
// Mirror pattern ProductPriceHistory tapi snapshot lengkap (min_qty, price,
// target_type, member whitelist sebagai JSON).
//
// action values: "create" | "update" | "delete"
// status values: "active"  | "inactive"
// MemberIDs: JSON array string member.id (snapshot whitelist saat itu).
// Pakai datatypes.JSON supaya GORM bisa serialize tanpa custom marshaler.
type ProductPriceTierHistory struct {
	ID         string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	TierID     string         `gorm:"type:varchar(36);not null;index" json:"tier_id"`
	ProductID  string         `gorm:"type:varchar(36);not null;index" json:"product_id"`
	MinQty     int            `gorm:"type:int;not null" json:"min_qty"`
	Price      float64        `gorm:"type:decimal(15,2);not null" json:"price"`
	TargetType string         `gorm:"type:varchar(20);not null" json:"target_type"`
	MemberIDs  datatypes.JSON `gorm:"type:json;null" json:"member_ids,omitempty"`
	Note       string         `gorm:"type:varchar(200);null" json:"note,omitempty"`
	ExpiresAt  *time.Time     `gorm:"column:expires_at;type:datetime;null" json:"expires_at,omitempty"`
	Status     string         `gorm:"type:varchar(20);not null;default:'active';index" json:"status"`
	Action     string         `gorm:"type:varchar(20);not null" json:"action"`
	StartDate  time.Time      `gorm:"not null;default:current_timestamp()" json:"start_date"`
	EndDate    *time.Time     `gorm:"null" json:"end_date,omitempty"`
	ChangedBy  *string        `gorm:"type:varchar(36);null" json:"changed_by,omitempty"`
	CreatedAt  time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (ProductPriceTierHistory) TableName() string { return "product_price_tier_history" }
