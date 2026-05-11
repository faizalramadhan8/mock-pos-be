package entity

import (
	"time"

	"gorm.io/gorm"
)

// ExpenseCategory — chart of accounts kategori pengeluaran. Seeded 12
// kategori standar di migration 000027 (is_system=1). Owner bisa tambah
// custom kategori (is_system=0).
type ExpenseCategory struct {
	ID        string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	IsSystem  bool      `gorm:"type:tinyint(1);not null;default:0" json:"is_system"`
	IsActive  bool      `gorm:"type:tinyint(1);not null;default:1" json:"is_active"`
	SortOrder int       `gorm:"type:int;not null;default:0" json:"sort_order"`
	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (ExpenseCategory) TableName() string { return "expense_categories" }

// Expense — satu baris pengeluaran operasional toko (di luar pembelian dari
// supplier, yang sudah di-track terpisah via purchase_invoices). Dipakai
// untuk Laporan Laba Rugi: Untung = Omzet - HPP - SUM(expenses).
//
// employee_name optional text (bukan FK users) — bisa catat pegawai gudang
// yang nggak punya akun. FE auto-suggest dari users table via datalist.
type Expense struct {
	ID            string           `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	CategoryID    string           `gorm:"type:varchar(36);not null" json:"category_id"`
	Category      *ExpenseCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	ExpenseDate   time.Time        `gorm:"type:date;not null" json:"expense_date"`
	Description   string           `gorm:"type:varchar(255);not null" json:"description"`
	Amount        float64          `gorm:"type:decimal(15,2);not null;default:0" json:"amount"`
	EmployeeName  string           `gorm:"type:varchar(100);null" json:"employee_name,omitempty"`
	PaymentMethod string           `gorm:"type:varchar(20);not null;default:'cash'" json:"payment_method"`
	Note          string           `gorm:"type:text;null" json:"note,omitempty"`
	CreatedBy     string           `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt     time.Time        `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt     time.Time        `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt     gorm.DeletedAt   `gorm:"index" json:"deleted_at,omitempty"`
}

func (Expense) TableName() string { return "expenses" }
