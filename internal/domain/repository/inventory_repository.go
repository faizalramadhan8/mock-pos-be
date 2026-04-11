package repository

import (
	"context"
	"time"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type StockMovementRepository struct {
	DB *gorm.DB
}

func NewStockMovementRepository(ctx context.Context, db *gorm.DB) *StockMovementRepository {
	return &StockMovementRepository{DB: db}
}

func (r *StockMovementRepository) FindAll(movementType string, limit, offset int) ([]entity.StockMovement, int64, error) {
	var movements []entity.StockMovement
	var total int64

	query := r.DB.Model(&entity.StockMovement{})
	if movementType != "" && movementType != "all" {
		query = query.Where("type = ?", movementType)
	}
	query.Count(&total)

	if err := query.Preload("Product").Preload("Supplier").Order("created_at DESC").Limit(limit).Offset(offset).Find(&movements).Error; err != nil {
		return nil, 0, err
	}
	return movements, total, nil
}

func (r *StockMovementRepository) FindByProductID(productID string) ([]entity.StockMovement, error) {
	var movements []entity.StockMovement
	if err := r.DB.Preload("Product").Preload("Supplier").Where("product_id = ?", productID).Order("created_at DESC").Find(&movements).Error; err != nil {
		return nil, err
	}
	return movements, nil
}

func (r *StockMovementRepository) Create(movement *entity.StockMovement) error {
	return r.DB.Create(movement).Error
}

func (r *StockMovementRepository) Update(movement *entity.StockMovement) error {
	return r.DB.Save(movement).Error
}

func (r *StockMovementRepository) FindByID(id string) (*entity.StockMovement, error) {
	var movement entity.StockMovement
	if err := r.DB.Where("id = ?", id).First(&movement).Error; err != nil {
		return nil, err
	}
	return &movement, nil
}

type StockBatchRepository struct {
	DB *gorm.DB
}

func NewStockBatchRepository(ctx context.Context, db *gorm.DB) *StockBatchRepository {
	return &StockBatchRepository{DB: db}
}

func (r *StockBatchRepository) FindAll(limit, offset int) ([]entity.StockBatch, int64, error) {
	var batches []entity.StockBatch
	var total int64

	r.DB.Model(&entity.StockBatch{}).Count(&total)

	if err := r.DB.Preload("Product").Order("created_at DESC").Limit(limit).Offset(offset).Find(&batches).Error; err != nil {
		return nil, 0, err
	}
	return batches, total, nil
}

func (r *StockBatchRepository) FindByProductID(productID string) ([]entity.StockBatch, error) {
	var batches []entity.StockBatch
	if err := r.DB.Where("product_id = ? AND quantity > 0", productID).Order("received_at ASC").Find(&batches).Error; err != nil {
		return nil, err
	}
	return batches, nil
}

func (r *StockBatchRepository) FindExpiring(withinDays int) ([]entity.StockBatch, error) {
	var batches []entity.StockBatch
	deadline := time.Now().AddDate(0, 0, withinDays).Format("2006-01-02")
	if err := r.DB.Preload("Product").Where("expiry_date IS NOT NULL AND expiry_date <= ? AND quantity > 0", deadline).Order("expiry_date ASC").Find(&batches).Error; err != nil {
		return nil, err
	}
	return batches, nil
}

func (r *StockBatchRepository) Create(batch *entity.StockBatch) error {
	return r.DB.Create(batch).Error
}

func (r *StockBatchRepository) Update(batch *entity.StockBatch) error {
	return r.DB.Save(batch).Error
}

func (r *StockBatchRepository) FindByID(id string) (*entity.StockBatch, error) {
	var batch entity.StockBatch
	if err := r.DB.Where("id = ?", id).First(&batch).Error; err != nil {
		return nil, err
	}
	return &batch, nil
}
