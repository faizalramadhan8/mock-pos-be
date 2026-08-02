package entity

import "time"

// EcomActivityLog — Sprint 5 Chunk 9 (2 Aug 2026).
// Audit trail per aksi admin di ecom panel.
type EcomActivityLog struct {
	ID          string    `gorm:"type:varchar(36);primary_key" json:"id"`
	AdminID     string    `gorm:"column:admin_id;type:varchar(36);not null" json:"admin_id"`
	AdminName   string    `gorm:"-" json:"admin_name,omitempty"` // populated in usecase
	Action      string    `gorm:"type:varchar(50);not null" json:"action"`
	Target      *string   `gorm:"type:varchar(100);null" json:"target,omitempty"`
	Description string    `gorm:"type:varchar(500);not null" json:"description"`
	Meta        *string   `gorm:"type:json;null" json:"meta,omitempty"`
	CreatedAt   time.Time `gorm:"default:current_timestamp()" json:"created_at"`
}

func (EcomActivityLog) TableName() string { return "ecom_activity_log" }
