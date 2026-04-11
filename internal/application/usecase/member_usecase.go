package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type MemberService struct {
	Log  *zerolog.Logger
	Repo *repository.MemberRepository
}

func NewMemberService(ctx context.Context, db *gorm.DB) *MemberService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &MemberService{
		Log:  logger,
		Repo: repository.NewMemberRepository(ctx, db),
	}
}

func (s *MemberService) GetAll(search string, page, limit int) ([]dto.MemberResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	members, total, err := s.Repo.FindAll(search, limit, offset)
	if err != nil {
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch members"}
	}

	var result []dto.MemberResponse
	for _, m := range members {
		result = append(result, dto.MemberResponse{
			ID:        m.ID,
			Name:      m.Name,
			Phone:     m.Phone,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, total, nil
}

func (s *MemberService) Create(req dto.CreateMemberRequest) (*dto.MemberResponse, *dto.ApiError) {
	member := &entity.Member{
		ID:    uuid.New().String(),
		Name:  req.Name,
		Phone: req.Phone,
	}

	if err := s.Repo.Create(member); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create member")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create member"}
	}

	return &dto.MemberResponse{
		ID:        member.ID,
		Name:      member.Name,
		Phone:     member.Phone,
		CreatedAt: member.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *MemberService) SearchByPhone(phone string) (*dto.MemberResponse, *dto.ApiError) {
	member, err := s.Repo.FindByPhone(phone)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Member not found"}
	}
	return &dto.MemberResponse{
		ID:        member.ID,
		Name:      member.Name,
		Phone:     member.Phone,
		CreatedAt: member.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *MemberService) Delete(id string) *dto.ApiError {
	if err := s.Repo.Delete(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete member"}
	}
	return nil
}
