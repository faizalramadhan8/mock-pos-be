package entity

import "time"

type Refund struct {
	ID        string       `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	OrderID   string       `gorm:"type:varchar(36);not null" json:"order_id"`
	Order     *Order       `gorm:"foreignKey:OrderID" json:"order,omitempty"`
	Items     []RefundItem `gorm:"foreignKey:RefundID" json:"items,omitempty"`
	Amount    float64      `gorm:"type:decimal(15,2);not null;default:0" json:"amount"`
	Reason    string       `gorm:"type:text;null" json:"reason,omitempty"`
	CreatedBy string       `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt time.Time    `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (Refund) TableName() string { return "refunds" }

type RefundItem struct {
	ID           string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	RefundID     string    `gorm:"type:varchar(36);not null" json:"refund_id"`
	ProductID    string    `gorm:"type:varchar(36);not null" json:"product_id"`
	Name         string    `gorm:"type:varchar(200);not null" json:"name"`
	Quantity     int       `gorm:"type:int;not null;default:1" json:"quantity"`
	UnitType     string    `gorm:"type:varchar(20);not null;default:'individual'" json:"unit_type"`
	UnitPrice    float64   `gorm:"type:decimal(15,2);not null;default:0" json:"unit_price"`
	RefundAmount float64   `gorm:"type:decimal(15,2);not null;default:0" json:"refund_amount"`
	CreatedAt    time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (RefundItem) TableName() string { return "refund_items" }
