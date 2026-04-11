package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type PushRepository struct {
	DB *gorm.DB
}

func NewPushRepository(ctx context.Context, db *gorm.DB) *PushRepository {
	return &PushRepository{DB: db}
}

func (r *PushRepository) FindAll() ([]entity.PushSubscription, error) {
	var subs []entity.PushSubscription
	if err := r.DB.Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *PushRepository) FindByUserID(userID string) ([]entity.PushSubscription, error) {
	var subs []entity.PushSubscription
	if err := r.DB.Where("user_id = ?", userID).Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *PushRepository) FindByEndpoint(endpoint string) (*entity.PushSubscription, error) {
	var sub entity.PushSubscription
	if err := r.DB.Where("endpoint = ?", endpoint).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *PushRepository) Create(sub *entity.PushSubscription) error {
	return r.DB.Create(sub).Error
}

func (r *PushRepository) Delete(id string) error {
	return r.DB.Delete(&entity.PushSubscription{}, "id = ?", id).Error
}

func (r *PushRepository) DeleteByEndpoint(endpoint string) error {
	return r.DB.Where("endpoint = ?", endpoint).Delete(&entity.PushSubscription{}).Error
}
