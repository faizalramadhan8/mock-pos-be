package entity

import (
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"gorm.io/gorm"
)

type User struct {
	ID          string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Email       string         `gorm:"type:varchar(100);not null;uniqueIndex" json:"email,omitempty"`
	PhoneNumber string         `gorm:"column:phone;type:varchar(20);null;uniqueIndex" json:"phone_number,omitempty"`
	FullName    string         `gorm:"column:fullname;type:varchar(100);not null" json:"full_name,omitempty"`
	Password    string         `gorm:"type:varchar(255);not null" json:"password,omitempty"`
	Role        enum.Role      `gorm:"type:enum('user','admin','superadmin','cashier','staff');default:'user';not null" json:"role,omitempty"`
	NIK         string         `gorm:"column:nik;type:varchar(50);null" json:"nik,omitempty"`
	DateOfBirth *time.Time     `gorm:"column:date_of_birth;type:date;null" json:"date_of_birth,omitempty"`
	IsActive    bool           `gorm:"column:is_active;type:tinyint(1);default:1;not null" json:"is_active"`
	CreatedAt   time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt   time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (User) TableName() string {
	return "users"
}
