package repository

import (
	"context"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type MemberRepository struct {
	DB *gorm.DB
}

func NewMemberRepository(ctx context.Context, db *gorm.DB) *MemberRepository {
	return &MemberRepository{DB: db}
}

func (r *MemberRepository) FindAll(search string, limit, offset int) ([]entity.Member, int64, error) {
	var members []entity.Member
	var total int64

	query := r.DB.Model(&entity.Member{})
	if search != "" {
		query = query.Where("name LIKE ? OR phone LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	query.Count(&total)

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&members).Error; err != nil {
		return nil, 0, err
	}
	return members, total, nil
}

func (r *MemberRepository) FindByPhone(phone string) (*entity.Member, error) {
	var member entity.Member
	if err := r.DB.Where("phone = ?", phone).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *MemberRepository) Create(member *entity.Member) error {
	return r.DB.Create(member).Error
}

func (r *MemberRepository) Delete(id string) error {
	return r.DB.Delete(&entity.Member{}, "id = ?", id).Error
}
