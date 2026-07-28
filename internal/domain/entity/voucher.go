package entity

import (
	"time"

	"gorm.io/gorm"
)

// EcomVoucher — kode diskon untuk storefront. Kode CASE-INSENSITIVE
// (di-uppercase saat store/compare di service layer).
type EcomVoucher struct {
	ID          string   `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Code        string   `gorm:"type:varchar(50);not null;uniqueIndex" json:"code"`
	Description string   `gorm:"type:varchar(200);null" json:"description,omitempty"`
	Type        string   `gorm:"type:varchar(10);not null" json:"type"` // 'percent' | 'fixed'
	Value       float64  `gorm:"type:decimal(15,2);not null" json:"value"`
	MinSubtotal float64  `gorm:"column:min_subtotal;type:decimal(15,2);not null;default:0" json:"min_subtotal"`
	MaxDiscount *float64 `gorm:"column:max_discount;type:decimal(15,2);null" json:"max_discount,omitempty"`
	UsageLimit  int      `gorm:"column:usage_limit;type:int;not null;default:0" json:"usage_limit"`
	UsedCount   int      `gorm:"column:used_count;type:int;not null;default:0" json:"used_count"`
	StartsAt    *time.Time `gorm:"column:starts_at;null" json:"starts_at,omitempty"`
	ExpiresAt   *time.Time `gorm:"column:expires_at;null" json:"expires_at,omitempty"`
	IsActive    bool     `gorm:"column:is_active;type:tinyint(1);not null;default:1" json:"is_active"`
	CreatedAt   time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt   time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (EcomVoucher) TableName() string { return "ecom_vouchers" }
