package repository

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type DeviceRepository struct {
	DB *gorm.DB
}

func NewDeviceRepository(ctx context.Context, db *gorm.DB) *DeviceRepository {
	return &DeviceRepository{DB: db}
}

func (r *DeviceRepository) FindByUserAndFingerprint(userID, fingerprint string) (*entity.TrustedDevice, error) {
	var d entity.TrustedDevice
	if err := r.DB.Where("user_id = ? AND fingerprint = ?", userID, fingerprint).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepository) FindByApprovalCode(code string) (*entity.TrustedDevice, error) {
	var d entity.TrustedDevice
	if err := r.DB.Where("approval_code = ?", code).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepository) FindLatestPending() (*entity.TrustedDevice, error) {
	var d entity.TrustedDevice
	if err := r.DB.Where("status = ?", entity.DeviceStatusPending).
		Order("created_at DESC").First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepository) FindByUser(userID string) ([]entity.TrustedDevice, error) {
	var out []entity.TrustedDevice
	if err := r.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *DeviceRepository) Create(d *entity.TrustedDevice) error {
	return r.DB.Create(d).Error
}

func (r *DeviceRepository) Update(d *entity.TrustedDevice) error {
	return r.DB.Save(d).Error
}

func (r *DeviceRepository) Delete(id string) error {
	return r.DB.Delete(&entity.TrustedDevice{}, "id = ?", id).Error
}

func (r *DeviceRepository) MarkUsed(id string) error {
	now := time.Now()
	return r.DB.Model(&entity.TrustedDevice{}).Where("id = ?", id).
		Update("last_used_at", now).Error
}
