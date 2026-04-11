package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type CashSessionRepository struct {
	DB *gorm.DB
}

func NewCashSessionRepository(ctx context.Context, db *gorm.DB) *CashSessionRepository {
	return &CashSessionRepository{DB: db}
}

func (r *CashSessionRepository) FindAll() ([]entity.CashSession, error) {
	var sessions []entity.CashSession
	if err := r.DB.Order("created_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *CashSessionRepository) FindByID(id string) (*entity.CashSession, error) {
	var session entity.CashSession
	if err := r.DB.Where("id = ?", id).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *CashSessionRepository) FindOpenSession() (*entity.CashSession, error) {
	var session entity.CashSession
	if err := r.DB.Where("closed_at IS NULL").Order("opened_at DESC").First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *CashSessionRepository) Create(session *entity.CashSession) error {
	return r.DB.Create(session).Error
}

func (r *CashSessionRepository) Update(session *entity.CashSession) error {
	return r.DB.Save(session).Error
}

func (r *CashSessionRepository) Close(id string, data map[string]interface{}) error {
	return r.DB.Model(&entity.CashSession{}).Where("id = ?", id).Updates(data).Error
}
