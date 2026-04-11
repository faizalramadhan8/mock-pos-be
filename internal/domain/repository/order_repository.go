package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type OrderRepository struct {
	DB *gorm.DB
}

func NewOrderRepository(ctx context.Context, db *gorm.DB) *OrderRepository {
	return &OrderRepository{DB: db}
}

func (r *OrderRepository) FindAll(status string, limit, offset int) ([]entity.Order, int64, error) {
	var orders []entity.Order
	var total int64

	query := r.DB.Model(&entity.Order{})
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	query.Count(&total)

	if err := query.Preload("Items").Order("created_at DESC").Limit(limit).Offset(offset).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *OrderRepository) FindByID(id string) (*entity.Order, error) {
	var order entity.Order
	if err := r.DB.Preload("Items").Where("id = ?", id).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) FindByDateRange(startDate, endDate string) ([]entity.Order, error) {
	var orders []entity.Order
	if err := r.DB.Preload("Items").Where("DATE(created_at) BETWEEN ? AND ?", startDate, endDate).Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *OrderRepository) Create(order *entity.Order) error {
	return r.DB.Create(order).Error
}

func (r *OrderRepository) Update(order *entity.Order) error {
	return r.DB.Save(order).Error
}

func (r *OrderRepository) GetRevenueStats() (float64, int64, error) {
	var result struct {
		Total float64
		Count int64
	}
	err := r.DB.Model(&entity.Order{}).
		Where("status = 'completed'").
		Select("COALESCE(SUM(total), 0) as total, COUNT(*) as count").
		Scan(&result).Error
	return result.Total, result.Count, err
}
