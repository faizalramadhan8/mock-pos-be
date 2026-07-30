package entity

import "time"

// EcomRestockAlert — Sprint 3 #16. Customer subscribe alert kalau produk yang
// habis kembali tersedia. Trigger notif saat stock_ecom naik dari 0 → >0.
type EcomRestockAlert struct {
	ID         string     `gorm:"type:varchar(36);primary_key" json:"id"`
	UserID     string     `gorm:"column:user_id;type:varchar(36);not null;index" json:"user_id"`
	ProductID  string     `gorm:"column:product_id;type:varchar(36);not null;index" json:"product_id"`
	NotifiedAt *time.Time `gorm:"column:notified_at;type:datetime;null" json:"notified_at,omitempty"`
	CreatedAt  time.Time  `gorm:"default:current_timestamp()" json:"created_at"`
}

func (EcomRestockAlert) TableName() string { return "ecom_restock_alerts" }
