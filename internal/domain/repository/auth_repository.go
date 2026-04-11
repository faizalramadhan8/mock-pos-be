package repository

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type AuthRepository struct {
	DB *gorm.DB
}

func NewAuthRepository(ctx context.Context, db *gorm.DB) *AuthRepository {
	return &AuthRepository{
		DB: db,
	}
}

func (r *AuthRepository) FindByEmail(email string) (*entity.User, error) {
	var user entity.User
	if err := r.DB.First(&user, "email = ?", email).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindByID(id string) (*entity.User, error) {
	var user entity.User
	if err := r.DB.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) Create(user *entity.User) error {
	return r.DB.Create(user).Error
}

func (r *AuthRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.DB.Model(&entity.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AuthRepository) ExistsByPhone(phone string) (bool, error) {
	var count int64
	err := r.DB.Model(&entity.User{}).Where("phone = ?", phone).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AuthRepository) FindAll() ([]entity.User, error) {
	var users []entity.User
	if err := r.DB.Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *AuthRepository) Update(user *entity.User) error {
	return r.DB.Save(user).Error
}

func (r *AuthRepository) Delete(id string) error {
	return r.DB.Delete(&entity.User{}, "id = ?", id).Error
}
