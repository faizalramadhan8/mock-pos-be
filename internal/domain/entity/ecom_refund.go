package entity

import "time"

// EcomRefund — Sprint 4 Chunk 2 (31 Jul 2026).
// Track refund actual yang sudah dilakukan Bu Santi (out-of-band transfer)
// + item yang direstok. Multi-row per order untuk cover partial refund.
type EcomRefund struct {
	ID             string    `gorm:"type:varchar(36);primary_key" json:"id"`
	OrderID        string    `gorm:"column:order_id;type:varchar(36);not null;index" json:"order_id"`
	ComplaintID    *string   `gorm:"column:complaint_id;type:varchar(36);null;index" json:"complaint_id,omitempty"`
	Amount         float64   `gorm:"type:decimal(15,2);not null" json:"amount"`
	Method         string    `gorm:"type:varchar(20);not null" json:"method"`
	Note           *string   `gorm:"type:text;null" json:"note,omitempty"`
	RestockedItems *string   `gorm:"column:restocked_items;type:json;null" json:"-"`
	RefundedBy     string    `gorm:"column:refunded_by;type:varchar(36);not null" json:"refunded_by"`
	RefundedAt     time.Time `gorm:"column:refunded_at;default:current_timestamp()" json:"refunded_at"`
	CreatedAt      time.Time `gorm:"default:current_timestamp()" json:"created_at"`
}

func (EcomRefund) TableName() string { return "ecom_refunds" }
