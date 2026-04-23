package entity

import (
	"time"
	"gorm.io/gorm"
)

type Member struct {
	ID           string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Name         string         `gorm:"type:varchar(200);not null" json:"name"`
	Phone        string         `gorm:"type:varchar(20);not null" json:"phone"`
	Address      string         `gorm:"type:text;null" json:"address,omitempty"`
	// MemberNumber is a pointer so NULL-when-empty plays well with MySQL's
	// UNIQUE constraint — multiple NULLs are allowed, but multiple empty
	// strings are not. The kasir "Tambah Member" form does not require a
	// number, so most members hit this path.
	MemberNumber *string        `gorm:"column:member_number;type:varchar(50);null;uniqueIndex" json:"member_number,omitempty"`
	CreatedAt    time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt    time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (Member) TableName() string { return "members" }

type CashSession struct {
	ID           string     `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Date         string     `gorm:"type:date;not null" json:"date"`
	OpeningCash  float64    `gorm:"type:decimal(15,2);not null;default:0" json:"opening_cash"`
	OpenedBy     string     `gorm:"type:varchar(200);not null" json:"opened_by"`
	OpenedAt     time.Time  `gorm:"not null;default:current_timestamp()" json:"opened_at"`
	ExpectedCash float64    `gorm:"type:decimal(15,2);null;default:0" json:"expected_cash"`
	ActualCash   float64    `gorm:"type:decimal(15,2);null;default:0" json:"actual_cash"`
	Difference   float64    `gorm:"type:decimal(15,2);null;default:0" json:"difference"`
	Notes        string     `gorm:"type:text;null" json:"notes,omitempty"`
	ClosedBy     string     `gorm:"type:varchar(200);null" json:"closed_by,omitempty"`
	ClosedAt     *time.Time `gorm:"null" json:"closed_at,omitempty"`
	CreatedAt    time.Time  `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt    time.Time  `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (CashSession) TableName() string { return "cash_sessions" }

type AuditEntry struct {
	ID        string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Action    string    `gorm:"type:varchar(50);not null" json:"action"`
	UserID    string    `gorm:"column:user_id;type:varchar(36);not null" json:"user_id"`
	UserName  string    `gorm:"column:user_name;type:varchar(200);not null" json:"user_name"`
	Details   string    `gorm:"type:text;null" json:"details,omitempty"`
	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (AuditEntry) TableName() string { return "audit_entries" }

type Settings struct {
	ID           string        `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	StoreName    string        `gorm:"type:varchar(200);not null;default:'Bakeshop'" json:"store_name"`
	StoreAddress string        `gorm:"type:text;null" json:"store_address,omitempty"`
	StorePhone   string        `gorm:"type:varchar(20);null" json:"store_phone,omitempty"`
	PPNRate      float64       `gorm:"column:ppn_rate;type:decimal(5,2);not null;default:11" json:"ppn_rate"`
	LabelWidth   int           `gorm:"column:label_width;type:int;not null;default:40" json:"label_width"`
	LabelHeight  int           `gorm:"column:label_height;type:int;not null;default:30" json:"label_height"`
	BankAccounts []BankAccount `gorm:"foreignKey:SettingsID" json:"bank_accounts,omitempty"`
	CreatedAt    time.Time     `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt    time.Time     `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (Settings) TableName() string { return "settings" }

type BankAccount struct {
	ID            string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	SettingsID    string    `gorm:"type:varchar(36);not null" json:"settings_id"`
	BankName      string    `gorm:"type:varchar(100);not null" json:"bank_name"`
	AccountNumber string    `gorm:"type:varchar(50);not null" json:"account_number"`
	AccountHolder string    `gorm:"type:varchar(200);not null" json:"account_holder"`
	CreatedAt     time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (BankAccount) TableName() string { return "bank_accounts" }
