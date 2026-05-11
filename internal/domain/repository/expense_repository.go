package repository

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type ExpenseRepository struct {
	DB *gorm.DB
}

func NewExpenseRepository(ctx context.Context, db *gorm.DB) *ExpenseRepository {
	return &ExpenseRepository{DB: db}
}

// ─── Category ────────────────────────────────────────────────────────────

func (r *ExpenseRepository) FindAllCategories(includeInactive bool) ([]entity.ExpenseCategory, error) {
	var cats []entity.ExpenseCategory
	q := r.DB.Model(&entity.ExpenseCategory{})
	if !includeInactive {
		q = q.Where("is_active = ?", 1)
	}
	if err := q.Order("sort_order ASC, name ASC").Find(&cats).Error; err != nil {
		return nil, err
	}
	return cats, nil
}

func (r *ExpenseRepository) FindCategoryByID(id string) (*entity.ExpenseCategory, error) {
	var c entity.ExpenseCategory
	if err := r.DB.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ExpenseRepository) CreateCategory(c *entity.ExpenseCategory) error {
	return r.DB.Create(c).Error
}

func (r *ExpenseRepository) UpdateCategory(c *entity.ExpenseCategory) error {
	return r.DB.Save(c).Error
}

// ─── Expense ─────────────────────────────────────────────────────────────

// FindAll — list dengan filter periode + kategori. Sort by expense_date DESC
// (yang terbaru di atas). Preload Category supaya FE tidak perlu join lagi.
func (r *ExpenseRepository) FindAll(from, to, categoryID string, limit, offset int) ([]entity.Expense, int64, error) {
	var exps []entity.Expense
	var total int64

	q := r.DB.Model(&entity.Expense{})
	if from != "" {
		q = q.Where("expense_date >= ?", from)
	}
	if to != "" {
		q = q.Where("expense_date <= ?", to)
	}
	if categoryID != "" {
		q = q.Where("category_id = ?", categoryID)
	}
	q.Count(&total)

	if err := q.Preload("Category").
		Order("expense_date DESC, created_at DESC").
		Limit(limit).Offset(offset).Find(&exps).Error; err != nil {
		return nil, 0, err
	}
	return exps, total, nil
}

func (r *ExpenseRepository) FindByID(id string) (*entity.Expense, error) {
	var e entity.Expense
	if err := r.DB.Preload("Category").Where("id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ExpenseRepository) Create(e *entity.Expense) error {
	return r.DB.Create(e).Error
}

func (r *ExpenseRepository) Update(e *entity.Expense) error {
	return r.DB.Save(e).Error
}

func (r *ExpenseRepository) Delete(id string) error {
	return r.DB.Delete(&entity.Expense{}, "id = ?", id).Error
}

// SumByCategory — total per kategori dalam periode. Dipakai untuk Laporan
// Laba Rugi (breakdown beban operasional per kategori).
type CategorySum struct {
	CategoryID   string  `gorm:"column:category_id"`
	CategoryName string  `gorm:"column:category_name"`
	Total        float64 `gorm:"column:total"`
	Count        int64   `gorm:"column:count"`
}

func (r *ExpenseRepository) SumByCategory(from, to string) ([]CategorySum, error) {
	var rows []CategorySum
	q := r.DB.Table("expenses e").
		Select("e.category_id, c.name as category_name, COALESCE(SUM(e.amount), 0) as total, COUNT(*) as count").
		Joins("LEFT JOIN expense_categories c ON c.id = e.category_id").
		Where("e.deleted_at IS NULL")
	if from != "" {
		q = q.Where("e.expense_date >= ?", from)
	}
	if to != "" {
		q = q.Where("e.expense_date <= ?", to)
	}
	if err := q.Group("e.category_id, c.name").
		Order("total DESC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// SumTotal — grand total semua pengeluaran dalam periode. Untuk Dashboard
// widget "Total Pengeluaran Bulan Ini" + Laba Rugi.
func (r *ExpenseRepository) SumTotal(from, to string) (float64, int64, error) {
	var result struct {
		Total float64
		Count int64
	}
	q := r.DB.Model(&entity.Expense{})
	if from != "" {
		q = q.Where("expense_date >= ?", from)
	}
	if to != "" {
		q = q.Where("expense_date <= ?", to)
	}
	err := q.Select("COALESCE(SUM(amount), 0) as total, COUNT(*) as count").Scan(&result).Error
	return result.Total, result.Count, err
}
