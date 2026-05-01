package entity

import (
	"time"
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
	CreatedAt     time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt     time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Product) TableName() string { return "products" }

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
