package repository

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type CapitalInjectionRepository struct {
	DB *gorm.DB
}

func NewCapitalInjectionRepository(ctx context.Context, db *gorm.DB) *CapitalInjectionRepository {
	return &CapitalInjectionRepository{DB: db}
}

// FindByRange returns injections within [from, to] inclusive. Both empty = all.
func (r *CapitalInjectionRepository) FindByRange(from, to time.Time) ([]entity.CapitalInjection, error) {
	var rows []entity.CapitalInjection
	q := r.DB.Order("injected_at DESC")
	if !from.IsZero() {
		q = q.Where("injected_at >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("injected_at <= ?", to)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *CapitalInjectionRepository) FindByID(id string) (*entity.CapitalInjection, error) {
	var row entity.CapitalInjection
	if err := r.DB.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *CapitalInjectionRepository) Create(row *entity.CapitalInjection) error {
	return r.DB.Create(row).Error
}

func (r *CapitalInjectionRepository) Update(row *entity.CapitalInjection) error {
	return r.DB.Save(row).Error
}

func (r *CapitalInjectionRepository) Delete(id string) error {
	return r.DB.Delete(&entity.CapitalInjection{}, "id = ?", id).Error
}
