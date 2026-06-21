package repository

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type ProductPriceTierRepository struct {
	DB *gorm.DB
}

func NewProductPriceTierRepository(ctx context.Context, db *gorm.DB) *ProductPriceTierRepository {
	return &ProductPriceTierRepository{DB: db}
}

// FindByProduct returns all tiers for a product, including member whitelist
// for member_specific tiers. Sorted by min_qty ASC for consistent UI order.
func (r *ProductPriceTierRepository) FindByProduct(productID string) ([]entity.ProductPriceTier, error) {
	var tiers []entity.ProductPriceTier
	if err := r.DB.Preload("Members").
		Where("product_id = ?", productID).
		Order("min_qty ASC").
		Find(&tiers).Error; err != nil {
		return nil, err
	}
	return tiers, nil
}

// FindByID returns a single tier with its members preloaded.
func (r *ProductPriceTierRepository) FindByID(id string) (*entity.ProductPriceTier, error) {
	var tier entity.ProductPriceTier
	if err := r.DB.Preload("Members").Where("id = ?", id).First(&tier).Error; err != nil {
		return nil, err
	}
	return &tier, nil
}

// Create inserts a new tier with its member whitelist atomically.
func (r *ProductPriceTierRepository) Create(tier *entity.ProductPriceTier) error {
	return r.DB.Create(tier).Error
}

// Update saves edits to an existing tier. Member whitelist is REPLACED
// (delete old links + insert new) via GORM's association replace pattern.
// Caller passes the desired final list in tier.Members.
func (r *ProductPriceTierRepository) Update(tier *entity.ProductPriceTier) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// Update scalar fields first. expires_at di-update walaupun NULL —
		// admin bisa change dari "1 hari" ke "tidak terbatas" (NULL).
		if err := tx.Model(&entity.ProductPriceTier{}).Where("id = ?", tier.ID).Updates(map[string]interface{}{
			"min_qty":     tier.MinQty,
			"price":       tier.Price,
			"target_type": tier.TargetType,
			"note":        tier.Note,
			"expires_at":  tier.ExpiresAt,
		}).Error; err != nil {
			return err
		}
		// Replace member whitelist: works whether new list is empty or not.
		if err := tx.Model(tier).Association("Members").Replace(tier.Members); err != nil {
			return err
		}
		return nil
	})
}

// Delete removes a tier (CASCADE will drop member links).
func (r *ProductPriceTierRepository) Delete(id string) error {
	return r.DB.Delete(&entity.ProductPriceTier{}, "id = ?", id).Error
}

// FindByProducts batch-loads tiers for multiple products at once. Used by
// the list endpoint to avoid N+1 when returning many products with tiers.
func (r *ProductPriceTierRepository) FindByProducts(productIDs []string) (map[string][]entity.ProductPriceTier, error) {
	if len(productIDs) == 0 {
		return map[string][]entity.ProductPriceTier{}, nil
	}
	var tiers []entity.ProductPriceTier
	if err := r.DB.Preload("Members").
		Where("product_id IN ?", productIDs).
		Order("product_id, min_qty ASC").
		Find(&tiers).Error; err != nil {
		return nil, err
	}
	out := make(map[string][]entity.ProductPriceTier, len(productIDs))
	for _, t := range tiers {
		out[t.ProductID] = append(out[t.ProductID], t)
	}
	return out, nil
}
