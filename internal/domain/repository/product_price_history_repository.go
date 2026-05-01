package repository

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type ProductPriceHistoryRepository struct {
	DB *gorm.DB
}

func NewProductPriceHistoryRepository(ctx context.Context, db *gorm.DB) *ProductPriceHistoryRepository {
	return &ProductPriceHistoryRepository{DB: db}
}

// FindByProduct returns all history rows for a product, newest first.
// Optional priceType filter ("" = all types).
func (r *ProductPriceHistoryRepository) FindByProduct(productID, priceType string) ([]entity.ProductPriceHistory, error) {
	var rows []entity.ProductPriceHistory
	q := r.DB.Where("product_id = ?", productID)
	if priceType != "" {
		q = q.Where("price_type = ?", priceType)
	}
	if err := q.Order("start_date DESC, created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// FindActive returns the active row for a (product, priceType), or nil if none.
func (r *ProductPriceHistoryRepository) FindActive(productID, priceType string) (*entity.ProductPriceHistory, error) {
	var row entity.ProductPriceHistory
	err := r.DB.Where("product_id = ? AND price_type = ? AND status = 'active'", productID, priceType).
		Order("start_date DESC").First(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *ProductPriceHistoryRepository) Create(row *entity.ProductPriceHistory) error {
	return r.DB.Create(row).Error
}

// CloseActive marks the currently-active row(s) for a (product, priceType) as
// inactive with end_date = at. Used right before inserting a new active row.
func (r *ProductPriceHistoryRepository) CloseActive(productID, priceType string, at time.Time) error {
	return r.DB.Model(&entity.ProductPriceHistory{}).
		Where("product_id = ? AND price_type = ? AND status = 'active'", productID, priceType).
		Updates(map[string]interface{}{"status": "inactive", "end_date": at}).Error
}
