package entity

import "time"

type PushSubscription struct {
	ID        string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	UserID    string    `gorm:"type:varchar(36);not null" json:"user_id"`
	Endpoint  string    `gorm:"type:text;not null" json:"endpoint"`
	P256dh    string    `gorm:"type:text;not null" json:"p256dh"`
	Auth      string    `gorm:"type:text;not null" json:"auth"`
	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (PushSubscription) TableName() string { return "push_subscriptions" }
