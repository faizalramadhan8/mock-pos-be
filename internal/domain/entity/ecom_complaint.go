package entity

import "time"

// EcomComplaint — customer submit komplain untuk order tertentu (rusak/salah/
// kurang). Admin bisa reply + resolve.
type EcomComplaint struct {
	ID          string     `gorm:"type:varchar(36);primary_key" json:"id"`
	OrderID     string     `gorm:"column:order_id;type:varchar(36);not null;index" json:"order_id"`
	UserID      string     `gorm:"column:user_id;type:varchar(36);not null;index" json:"user_id"`
	Reason      string     `gorm:"type:varchar(30);not null" json:"reason"`      // barang_rusak/barang_salah/barang_kurang/lainnya
	Description string     `gorm:"type:text;not null" json:"description"`
	Images      *string    `gorm:"type:json;null" json:"images,omitempty"`       // JSON array URL
	Status      string     `gorm:"type:varchar(20);not null;default:'open'" json:"status"` // open/in_review/resolved/rejected
	AdminReply  *string    `gorm:"column:admin_reply;type:text;null" json:"admin_reply,omitempty"`
	AdminID     *string    `gorm:"column:admin_id;type:varchar(36);null" json:"admin_id,omitempty"`
	CreatedAt   time.Time  `gorm:"default:current_timestamp()" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"default:current_timestamp()" json:"updated_at"`
	ResolvedAt  *time.Time `gorm:"column:resolved_at;type:datetime;null" json:"resolved_at,omitempty"`
}

func (EcomComplaint) TableName() string { return "ecom_complaints" }
