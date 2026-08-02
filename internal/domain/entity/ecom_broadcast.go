package entity

import "time"

// EcomBroadcast — Sprint 5 Chunk 6 (2 Aug 2026). History log push broadcast.
type EcomBroadcast struct {
	ID               string    `gorm:"type:varchar(36);primary_key" json:"id"`
	Title            string    `gorm:"type:varchar(100);not null" json:"title"`
	Body             string    `gorm:"type:varchar(500);not null" json:"body"`
	URL              *string   `gorm:"type:varchar(500);null" json:"url,omitempty"`
	DeliveredCount   int       `gorm:"column:delivered_count;type:int;not null;default:0" json:"delivered_count"`
	FailedCount      int       `gorm:"column:failed_count;type:int;not null;default:0" json:"failed_count"`
	TotalSubscribers int       `gorm:"column:total_subscribers;type:int;not null;default:0" json:"total_subscribers"`
	SentBy           string    `gorm:"column:sent_by;type:varchar(36);not null" json:"sent_by"`
	SentByName       string    `gorm:"-" json:"sent_by_name,omitempty"` // populated in usecase
	SentAt           time.Time `gorm:"column:sent_at;default:current_timestamp()" json:"sent_at"`
}

func (EcomBroadcast) TableName() string { return "ecom_broadcasts" }
