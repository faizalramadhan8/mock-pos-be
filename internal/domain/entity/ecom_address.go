package entity

import (
	"time"

	"gorm.io/gorm"
)

type EcomAddress struct {
	ID              string  `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	UserID          string  `gorm:"column:user_id;type:varchar(36);not null;index" json:"user_id"`
	Label           string  `gorm:"type:varchar(50);not null" json:"label"`
	RecipientName   string  `gorm:"column:recipient_name;type:varchar(200);not null" json:"recipient_name"`
	RecipientPhone  string  `gorm:"column:recipient_phone;type:varchar(20);not null" json:"recipient_phone"`
	Province        string  `gorm:"type:varchar(100);not null" json:"province"`
	City            string  `gorm:"type:varchar(100);not null" json:"city"`
	District        string  `gorm:"type:varchar(100);not null" json:"district"`
	Subdistrict     string  `gorm:"type:varchar(100);not null" json:"subdistrict"`
	Zipcode         string  `gorm:"type:varchar(10);not null" json:"zipcode"`
	StreetAddress   string  `gorm:"column:street_address;type:text;not null" json:"street_address"`
	Latitude        *float64 `gorm:"type:decimal(10,7);null" json:"latitude,omitempty"`
	Longitude       *float64 `gorm:"type:decimal(10,7);null" json:"longitude,omitempty"`
	Notes           *string  `gorm:"type:text;null" json:"notes,omitempty"`
	IsDefault       bool    `gorm:"column:is_default;type:tinyint(1);not null;default:0" json:"is_default"`
	CreatedAt       time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt       time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (EcomAddress) TableName() string { return "ecom_addresses" }
