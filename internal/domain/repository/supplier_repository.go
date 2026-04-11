package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type SupplierRepository struct {
	DB *gorm.DB
}

func NewSupplierRepository(ctx context.Context, db *gorm.DB) *SupplierRepository {
	return &SupplierRepository{DB: db}
}

func (r *SupplierRepository) FindAll(search string, limit, offset int) ([]entity.Supplier, int64, error) {
	var suppliers []entity.Supplier
	var total int64

	query := r.DB.Model(&entity.Supplier{})
	if search != "" {
		query = query.Where("name LIKE ? OR phone LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	query.Count(&total)

	if err := query.Order("name ASC").Limit(limit).Offset(offset).Find(&suppliers).Error; err != nil {
		return nil, 0, err
	}
	return suppliers, total, nil
}

func (r *SupplierRepository) FindByID(id string) (*entity.Supplier, error) {
	var supplier entity.Supplier
	if err := r.DB.Where("id = ?", id).First(&supplier).Error; err != nil {
		return nil, err
	}
	return &supplier, nil
}

func (r *SupplierRepository) Create(supplier *entity.Supplier) error {
	return r.DB.Create(supplier).Error
}

func (r *SupplierRepository) Update(supplier *entity.Supplier) error {
	return r.DB.Save(supplier).Error
}

func (r *SupplierRepository) Delete(id string) error {
	return r.DB.Delete(&entity.Supplier{}, "id = ?", id).Error
}
