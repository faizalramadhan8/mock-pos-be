package entity

import (
	"time"
	"gorm.io/gorm"
)

type Order struct {
	ID                 string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Items              []OrderItem    `gorm:"foreignKey:OrderID" json:"items,omitempty"`
	Subtotal           float64        `gorm:"type:decimal(15,2);not null;default:0" json:"subtotal"`
	PPNRate            float64        `gorm:"column:ppn_rate;type:decimal(5,2);not null;default:11" json:"ppn_rate"`
	PPN                float64        `gorm:"type:decimal(15,2);not null;default:0" json:"ppn"`
	Total              float64        `gorm:"type:decimal(15,2);not null;default:0" json:"total"`
	Payment            string         `gorm:"type:varchar(20);not null;default:'cash'" json:"payment"`
	Status             string         `gorm:"type:varchar(20);not null;default:'completed'" json:"status"`
	Customer           string         `gorm:"type:varchar(200);null" json:"customer,omitempty"`
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
	ID             string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	OrderID        string    `gorm:"type:varchar(36);not null" json:"order_id"`
	ProductID      string    `gorm:"type:varchar(36);not null" json:"product_id"`
	Name           string    `gorm:"type:varchar(200);not null" json:"name"`
	Quantity       int       `gorm:"type:int;not null;default:1" json:"quantity"`
	UnitType       string    `gorm:"type:varchar(20);not null;default:'individual'" json:"unit_type"`
	UnitPrice      float64   `gorm:"type:decimal(15,2);not null;default:0" json:"unit_price"`
	DiscountType   string    `gorm:"type:varchar(20);null" json:"discount_type,omitempty"`
	DiscountValue  float64   `gorm:"type:decimal(15,2);null;default:0" json:"discount_value"`
	DiscountAmount float64   `gorm:"type:decimal(15,2);null;default:0" json:"discount_amount"`
	CreatedAt      time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (OrderItem) TableName() string { return "order_items" }
