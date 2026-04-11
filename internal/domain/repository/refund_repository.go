package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type RefundRepository struct {
	DB *gorm.DB
}

func NewRefundRepository(ctx context.Context, db *gorm.DB) *RefundRepository {
	return &RefundRepository{DB: db}
}

func (r *RefundRepository) FindAll() ([]entity.Refund, error) {
	var refunds []entity.Refund
	if err := r.DB.Preload("Items").Order("created_at DESC").Find(&refunds).Error; err != nil {
		return nil, err
	}
	return refunds, nil
}

func (r *RefundRepository) FindByOrderID(orderID string) ([]entity.Refund, error) {
	var refunds []entity.Refund
	if err := r.DB.Preload("Items").Where("order_id = ?", orderID).Find(&refunds).Error; err != nil {
		return nil, err
	}
	return refunds, nil
}

func (r *RefundRepository) Create(refund *entity.Refund) error {
	return r.DB.Create(refund).Error
}
