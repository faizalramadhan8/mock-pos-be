package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type AuditRepository struct {
	DB *gorm.DB
}

func NewAuditRepository(ctx context.Context, db *gorm.DB) *AuditRepository {
	return &AuditRepository{DB: db}
}

func (r *AuditRepository) FindAll(limit, offset int) ([]entity.AuditEntry, int64, error) {
	var entries []entity.AuditEntry
	var total int64

	r.DB.Model(&entity.AuditEntry{}).Count(&total)

	if err := r.DB.Order("created_at DESC").Limit(limit).Offset(offset).Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (r *AuditRepository) Create(entry *entity.AuditEntry) error {
	return r.DB.Create(entry).Error
}
