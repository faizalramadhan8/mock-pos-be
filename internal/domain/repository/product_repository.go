package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type ProductRepository struct {
	DB *gorm.DB
}

func NewProductRepository(ctx context.Context, db *gorm.DB) *ProductRepository {
	return &ProductRepository{DB: db}
}

func (r *ProductRepository) FindAll(search, categoryID string, limit, offset int) ([]entity.Product, int64, error) {
	var products []entity.Product
	var total int64

	query := r.DB.Model(&entity.Product{})
	if search != "" {
		query = query.Where("name LIKE ? OR name_id LIKE ? OR sku LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}
	query.Count(&total)

	if err := query.Preload("Category").Order("created_at DESC").Limit(limit).Offset(offset).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *ProductRepository) FindByID(id string) (*entity.Product, error) {
	var product entity.Product
	if err := r.DB.Preload("Category").Where("id = ?", id).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) FindBySKU(sku string) (*entity.Product, error) {
	var product entity.Product
	if err := r.DB.Preload("Category").Where("sku = ?", sku).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) FindLowStock() ([]entity.Product, error) {
	var products []entity.Product
	if err := r.DB.Preload("Category").Where("stock <= min_stock AND is_active = 1").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

func (r *ProductRepository) Create(product *entity.Product) error {
	return r.DB.Create(product).Error
}

func (r *ProductRepository) Update(product *entity.Product) error {
	return r.DB.Save(product).Error
}

func (r *ProductRepository) AdjustStock(id string, delta int) error {
	return r.DB.Model(&entity.Product{}).Where("id = ?", id).
		Update("stock", gorm.Expr("stock + ?", delta)).Error
}

func (r *ProductRepository) CountActive() (int64, error) {
	var count int64
	err := r.DB.Model(&entity.Product{}).Where("is_active = 1").Count(&count).Error
	return count, err
}

func (r *ProductRepository) ExistsBySKU(sku string) (bool, error) {
	var count int64
	err := r.DB.Model(&entity.Product{}).Where("sku = ?", sku).Count(&count).Error
	return count > 0, err
}

// Delete performs a soft delete (sets deleted_at) so order history can still
// reference the product by id + cached name snapshot.
func (r *ProductRepository) Delete(id string) error {
	return r.DB.Delete(&entity.Product{}, "id = ?", id).Error
}
