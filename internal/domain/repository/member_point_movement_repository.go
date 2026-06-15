package repository

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type MemberPointMovementRepository struct {
	DB *gorm.DB
}

func NewMemberPointMovementRepository(ctx context.Context, db *gorm.DB) *MemberPointMovementRepository {
	return &MemberPointMovementRepository{DB: db}
}

// CreateTx inserts a movement row using the supplied transaction. Always
// pair with MemberRepository.UpdatePointsTx in the same tx for consistency.
func (r *MemberPointMovementRepository) CreateTx(tx *gorm.DB, m *entity.MemberPointMovement) error {
	return tx.Create(m).Error
}

// FindByMember returns history (newest first) for a single member.
// Limit caps the result set; 0 = no limit.
func (r *MemberPointMovementRepository) FindByMember(memberID string, limit int) ([]entity.MemberPointMovement, error) {
	q := r.DB.Where("member_id = ?", memberID).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	var rows []entity.MemberPointMovement
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
