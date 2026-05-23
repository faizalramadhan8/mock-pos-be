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

func (r *ProductRepository) FindAll(search, categoryID, supplierID string, limit, offset int) ([]entity.Product, int64, error) {
	var products []entity.Product
	var total int64

	query := r.DB.Model(&entity.Product{})
	if search != "" {
		query = query.Where("name LIKE ? OR name_id LIKE ? OR sku LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if categoryID != "" {
		query = query.Where("category_id = ?", categoryID)
	}
	if supplierID != "" {
		query = query.Where("supplier_id = ?", supplierID)
	}
	query.Count(&total)

	if err := query.Preload("Category").Preload("Supplier").Order("created_at DESC").Limit(limit).Offset(offset).Find(&products).Error; err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

func (r *ProductRepository) FindByID(id string) (*entity.Product, error) {
	var product entity.Product
	if err := r.DB.Preload("Category").Preload("Supplier").Where("id = ?", id).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) FindBySKU(sku string) (*entity.Product, error) {
	var product entity.Product
	if err := r.DB.Preload("Category").Preload("Supplier").Where("sku = ?", sku).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *ProductRepository) FindLowStock() ([]entity.Product, error) {
	var products []entity.Product
	if err := r.DB.Preload("Category").Preload("Supplier").Where("stock <= min_stock AND is_active = 1").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

func (r *ProductRepository) Create(product *entity.Product) error {
	return r.DB.Create(product).Error
}

func (r *ProductRepository) Update(product *entity.Product) error {
	// CRITICAL: nil-kan preloaded association objects sebelum Save.
	// FindByID Preload("Category") + Preload("Supplier"), jadi product struct
	// punya 2 representasi FK: (1) CategoryID string, (2) Category pointer ke
	// object lama. GORM Save akan auto-resync FK dari association object,
	// menimpa CategoryID baru yang sudah di-set di usecase. Hasilnya update
	// kategori (atau supplier) tampak sukses ke FE tapi DB tetap nilai lama.
	// Nil-out di sini memastikan GORM hanya pakai FK string yang sudah diset.
	product.Category = nil
	product.Supplier = nil
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

// FindMaxSKUNumberByPrefix returns the highest numeric suffix among SKUs that
// start with "<prefix>-". Pakai Unscoped() supaya ikut hitung soft-deleted
// rows — penting karena unique constraint di tabel cek SEMUA row termasuk
// yang ke-soft-delete. Tanpa Unscoped, FE auto-gen bisa balikkan number yang
// "stuck" di soft-deleted row → 1062 duplicate entry error.
func (r *ProductRepository) FindMaxSKUNumberByPrefix(prefix string) (int, error) {
	var maxNum struct{ MaxNum *int }
	pattern := prefix + "-%"
	// Extract numeric part after the "-": CAST(SUBSTRING_INDEX(sku, '-', -1) AS UNSIGNED)
	err := r.DB.
		Unscoped().
		Model(&entity.Product{}).
		Where("sku LIKE ?", pattern).
		Select("MAX(CAST(SUBSTRING_INDEX(sku, '-', -1) AS UNSIGNED)) as max_num").
		Scan(&maxNum).Error
	if err != nil {
		return 0, err
	}
	if maxNum.MaxNum == nil {
		return 0, nil
	}
	return *maxNum.MaxNum, nil
}
