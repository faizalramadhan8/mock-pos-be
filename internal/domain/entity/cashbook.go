package entity

import "time"

// CashbookOpeningBalance — saldo awal kas per bulan. Owner input manual
// di awal periode (lihat catatan migrasi 000035). Dipakai laporan Arus Kas
// (cash basis accounting) sebagai starting point sebelum aggregate
// transaksi masuk/keluar bulan tersebut.
type CashbookOpeningBalance struct {
	ID        string    `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Year      int       `gorm:"type:int;not null" json:"year"`
	Month     int       `gorm:"type:int;not null" json:"month"`
	Balance   float64   `gorm:"type:decimal(15,2);not null" json:"balance"`
	Note      string    `gorm:"type:text;null" json:"note,omitempty"`
	CreatedBy string    `gorm:"column:created_by;type:varchar(36);not null" json:"created_by"`
	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (CashbookOpeningBalance) TableName() string { return "cashbook_opening_balances" }
