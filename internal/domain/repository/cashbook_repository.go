package repository

import (
	"context"
	"errors"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type CashbookRepository struct {
	DB *gorm.DB
}

func NewCashbookRepository(ctx context.Context, db *gorm.DB) *CashbookRepository {
	return &CashbookRepository{DB: db}
}

// FindOpeningBalance returns saldo awal periode (year+month). Returns nil
// (no error) kalau belum di-set — caller treat sebagai 0 (clean slate).
func (r *CashbookRepository) FindOpeningBalance(year, month int) (*entity.CashbookOpeningBalance, error) {
	var ob entity.CashbookOpeningBalance
	err := r.DB.Where("year = ? AND month = ?", year, month).First(&ob).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ob, nil
}

// UpsertOpeningBalance — set/update saldo awal periode. Upsert pattern via
// FirstOrCreate kalau belum ada, lalu UPDATE kalau perlu adjust.
func (r *CashbookRepository) UpsertOpeningBalance(ob *entity.CashbookOpeningBalance) error {
	existing, err := r.FindOpeningBalance(ob.Year, ob.Month)
	if err != nil {
		return err
	}
	if existing == nil {
		return r.DB.Create(ob).Error
	}
	existing.Balance = ob.Balance
	existing.Note = ob.Note
	existing.CreatedBy = ob.CreatedBy
	return r.DB.Save(existing).Error
}

// FindAll — list semua opening balance, sorted DESC by year+month. Untuk
// audit/history admin lihat tren saldo awal per bulan.
func (r *CashbookRepository) FindAll() ([]entity.CashbookOpeningBalance, error) {
	var rows []entity.CashbookOpeningBalance
	if err := r.DB.Order("year DESC, month DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
