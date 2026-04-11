package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type SettingsRepository struct {
	DB *gorm.DB
}

func NewSettingsRepository(ctx context.Context, db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{DB: db}
}

func (r *SettingsRepository) Get() (*entity.Settings, error) {
	var settings entity.Settings
	if err := r.DB.Preload("BankAccounts").First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *SettingsRepository) Update(settings *entity.Settings) error {
	return r.DB.Save(settings).Error
}

func (r *SettingsRepository) AddBankAccount(account *entity.BankAccount) error {
	return r.DB.Create(account).Error
}

func (r *SettingsRepository) DeleteBankAccount(id string) error {
	return r.DB.Delete(&entity.BankAccount{}, "id = ?", id).Error
}
