package entity

import "time"

type StockMovement struct {
	ID            string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	ProductID     string    `gorm:"type:varchar(36);not null" json:"product_id"`
	Product       *Product  `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Type          string    `gorm:"type:varchar(10);not null" json:"type"`
	Quantity      int       `gorm:"type:int;not null;default:0" json:"quantity"`
	UnitType      string    `gorm:"type:varchar(20);not null;default:'individual'" json:"unit_type"`
	UnitPrice     float64   `gorm:"type:decimal(15,2);not null;default:0" json:"unit_price"`
	Note          string    `gorm:"type:text;null" json:"note,omitempty"`
	ExpiryDate    *string   `gorm:"type:date;null" json:"expiry_date,omitempty"`
	SupplierID    *string   `gorm:"type:varchar(36);null" json:"supplier_id,omitempty"`
	Supplier      *Supplier `gorm:"foreignKey:SupplierID" json:"supplier,omitempty"`
	PaymentTerms  string    `gorm:"type:varchar(20);null" json:"payment_terms,omitempty"`
	DueDate       *string   `gorm:"type:date;null" json:"due_date,omitempty"`
	PaymentStatus string    `gorm:"type:varchar(20);null;default:'paid'" json:"payment_status,omitempty"`
	CreatedBy     string    `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt     time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt     time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (StockMovement) TableName() string { return "stock_movements" }

type StockBatch struct {
	ID          string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	ProductID   string    `gorm:"type:varchar(36);not null" json:"product_id"`
	Product     *Product  `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Quantity    int       `gorm:"type:int;not null;default:0" json:"quantity"`
	ExpiryDate  *string   `gorm:"type:date;null" json:"expiry_date,omitempty"`
	ReceivedAt  time.Time `gorm:"default:current_timestamp()" json:"received_at"`
	Note        string    `gorm:"type:text;null" json:"note,omitempty"`
	BatchNumber string    `gorm:"type:varchar(50);not null" json:"batch_number"`
	CreatedAt   time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt   time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (StockBatch) TableName() string { return "stock_batches" }
