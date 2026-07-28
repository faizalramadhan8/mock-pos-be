package entity

import "time"

// EcomReview — 1 user × 1 produk × 1 review. Rating 1-5, comment optional.
type EcomReview struct {
	ID        string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	ProductID string    `gorm:"column:product_id;type:varchar(36);not null;index" json:"product_id"`
	UserID    string    `gorm:"column:user_id;type:varchar(36);not null" json:"user_id"`
	OrderID   *string   `gorm:"column:order_id;type:varchar(36);null" json:"order_id,omitempty"`
	Rating    int       `gorm:"type:tinyint;not null" json:"rating"`
	Comment   string    `gorm:"type:text;null" json:"comment,omitempty"`
	IsHidden  bool      `gorm:"column:is_hidden;type:tinyint(1);not null;default:0" json:"is_hidden"`
	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`

	// Preload user untuk display nama reviewer.
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (EcomReview) TableName() string { return "ecom_reviews" }
