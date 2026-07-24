package entity

import "time"

// EcomCartItem — per-user cart persistence untuk storefront ecom.
// Bu Santi 24 Jul 2026.
type EcomCartItem struct {
	ID        string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	UserID    string    `gorm:"column:user_id;type:varchar(36);not null;index" json:"user_id"`
	ProductID string    `gorm:"column:product_id;type:varchar(36);not null" json:"product_id"`
	Product   *Product  `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Quantity  int       `gorm:"type:int;not null;default:1" json:"quantity"`
	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (EcomCartItem) TableName() string { return "ecom_cart_items" }
