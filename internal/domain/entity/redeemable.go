package entity

import (
	"time"

	"gorm.io/gorm"
)

// RedeemableItem — barang khusus untuk tebus poin member, TERPISAH dari
// `products` table. Admin yang setup manual: nama, gambar, points_cost,
// stok awal. Saat customer tebus, `Stock` decrement + `Redeemed` increment.
//
// Lihat migration 000040.
type RedeemableItem struct {
	ID          string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Name        string         `gorm:"type:varchar(200);not null" json:"name"`
	Description string         `gorm:"type:varchar(500);null" json:"description,omitempty"`
	Image       string         `gorm:"type:varchar(500);null" json:"image,omitempty"`
	PointsCost  int            `gorm:"column:points_cost;type:int;not null" json:"points_cost"`
	Stock       int            `gorm:"type:int;not null;default:0" json:"stock"`
	Redeemed    int            `gorm:"type:int;not null;default:0" json:"redeemed"`
	IsActive    bool           `gorm:"column:is_active;type:tinyint(1);not null;default:1" json:"is_active"`
	CreatedAt   time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt   time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (RedeemableItem) TableName() string { return "redeemable_items" }
