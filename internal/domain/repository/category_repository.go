package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type CategoryRepository struct {
	DB *gorm.DB
}

func NewCategoryRepository(ctx context.Context, db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{DB: db}
}

func (r *CategoryRepository) FindAll() ([]entity.Category, error) {
	var categories []entity.Category
	if err := r.DB.Order("name ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *CategoryRepository) FindByID(id string) (*entity.Category, error) {
	var cat entity.Category
	if err := r.DB.Where("id = ?", id).First(&cat).Error; err != nil {
		return nil, err
	}
	return &cat, nil
}

func (r *CategoryRepository) Create(cat *entity.Category) error {
	return r.DB.Create(cat).Error
}

func (r *CategoryRepository) Update(cat *entity.Category) error {
	return r.DB.Save(cat).Error
}

func (r *CategoryRepository) Delete(id string) error {
	return r.DB.Delete(&entity.Category{}, "id = ?", id).Error
}
