package repository

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type RedeemableItemRepository struct {
	DB *gorm.DB
}

func NewRedeemableItemRepository(ctx context.Context, db *gorm.DB) *RedeemableItemRepository {
	return &RedeemableItemRepository{DB: db}
}

// FindAll returns all redeemable items (active + inactive, excluding soft-deleted).
// Admin UI uses this to show full catalog; POS filters is_active client-side.
func (r *RedeemableItemRepository) FindAll() ([]entity.RedeemableItem, error) {
	var items []entity.RedeemableItem
	if err := r.DB.Order("name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// FindActive returns only active items with stock > 0 — POS catalog scope.
// Tetap include item dengan stock=0 supaya admin bisa tau habis di POS.
func (r *RedeemableItemRepository) FindActive() ([]entity.RedeemableItem, error) {
	var items []entity.RedeemableItem
	if err := r.DB.Where("is_active = ?", true).Order("name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *RedeemableItemRepository) FindByID(id string) (*entity.RedeemableItem, error) {
	var item entity.RedeemableItem
	if err := r.DB.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *RedeemableItemRepository) Create(item *entity.RedeemableItem) error {
	return r.DB.Create(item).Error
}

func (r *RedeemableItemRepository) Update(item *entity.RedeemableItem) error {
	return r.DB.Save(item).Error
}

func (r *RedeemableItemRepository) Delete(id string) error {
	return r.DB.Delete(&entity.RedeemableItem{}, "id = ?", id).Error
}
