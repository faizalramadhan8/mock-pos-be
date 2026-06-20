package repository

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type ProductPriceTierHistoryRepository struct {
	DB *gorm.DB
}

func NewProductPriceTierHistoryRepository(ctx context.Context, db *gorm.DB) *ProductPriceTierHistoryRepository {
	return &ProductPriceTierHistoryRepository{DB: db}
}

// FindByProduct returns history rows for a product, newest first.
// Includes ALL tier history (active + inactive), so admin bisa lihat tier
// yang sudah dihapus juga.
func (r *ProductPriceTierHistoryRepository) FindByProduct(productID string) ([]entity.ProductPriceTierHistory, error) {
	var rows []entity.ProductPriceTierHistory
	if err := r.DB.Where("product_id = ?", productID).
		Order("start_date DESC, created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// FindByTier returns history rows for a specific tier (across versions).
func (r *ProductPriceTierHistoryRepository) FindByTier(tierID string) ([]entity.ProductPriceTierHistory, error) {
	var rows []entity.ProductPriceTierHistory
	if err := r.DB.Where("tier_id = ?", tierID).
		Order("start_date DESC, created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ProductPriceTierHistoryRepository) Create(row *entity.ProductPriceTierHistory) error {
	return r.DB.Create(row).Error
}

// CloseActive marks active row(s) for a tier as inactive with end_date=at.
// Pakai sebelum insert versi baru (Update) atau saat Delete tier.
func (r *ProductPriceTierHistoryRepository) CloseActive(tierID string, at time.Time) error {
	return r.DB.Model(&entity.ProductPriceTierHistory{}).
		Where("tier_id = ? AND status = 'active'", tierID).
		Updates(map[string]interface{}{"status": "inactive", "end_date": at}).Error
}
