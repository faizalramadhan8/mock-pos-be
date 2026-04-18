package entity

import "time"

type DeviceStatus string

const (
	DeviceStatusPending  DeviceStatus = "pending"
	DeviceStatusApproved DeviceStatus = "approved"
	DeviceStatusRejected DeviceStatus = "rejected"
)

type TrustedDevice struct {
	ID             string       `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	UserID         string       `gorm:"type:varchar(36);not null;index:uk_trusted_devices_user_fp,unique" json:"user_id"`
	Fingerprint    string       `gorm:"type:varchar(100);not null;index:uk_trusted_devices_user_fp,unique" json:"fingerprint"`
	Status         DeviceStatus `gorm:"type:enum('pending','approved','rejected');default:'pending';not null" json:"status"`
	ApprovalCode   string       `gorm:"column:approval_code;type:varchar(64);null" json:"approval_code,omitempty"`
	CodeExpiresAt  *time.Time   `gorm:"column:code_expires_at;null" json:"code_expires_at,omitempty"`
	Name           string       `gorm:"type:varchar(100);null" json:"name,omitempty"`
	UserAgent      string       `gorm:"column:user_agent;type:varchar(255);null" json:"user_agent,omitempty"`
	ApprovedAt     *time.Time   `gorm:"column:approved_at;null" json:"approved_at,omitempty"`
	LastUsedAt     *time.Time   `gorm:"column:last_used_at;null" json:"last_used_at,omitempty"`
	LastNotifiedAt *time.Time   `gorm:"column:last_notified_at;null" json:"last_notified_at,omitempty"`
	CreatedAt      time.Time    `gorm:"default:current_timestamp()" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"default:current_timestamp()" json:"updated_at"`
}

func (TrustedDevice) TableName() string {
	return "trusted_devices"
}
