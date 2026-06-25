package entity

import (
	"time"

	"gorm.io/gorm"
)

// CapitalInjection — setoran modal owner di luar penjualan. Tampil di
// laporan Arus Kas sebagai +ModalTambahan. Lihat migration 000043.
type CapitalInjection struct {
	ID     string  `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	Amount float64 `gorm:"type:decimal(15,2);not null" json:"amount"`
	// Type: 'injection' (setoran modal, +saldo) atau 'drawing' (prive, -saldo).
	// Default 'injection' untuk backward compat (existing data pre-migration 044).
	Type       string         `gorm:"type:varchar(20);not null;default:'injection'" json:"type"`
	Source     string         `gorm:"type:varchar(100);null" json:"source,omitempty"`
	Note       string         `gorm:"type:varchar(500);null" json:"note,omitempty"`
	InjectedAt time.Time      `gorm:"column:injected_at;type:datetime;not null;index" json:"injected_at"`
	CreatedBy  *string        `gorm:"column:created_by;type:varchar(36);null" json:"created_by,omitempty"`
	CreatedAt  time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt  time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (CapitalInjection) TableName() string { return "capital_injections" }
