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

type AuditService struct {
	Log  *zerolog.Logger
	Repo *repository.AuditRepository
}

func NewAuditService(ctx context.Context, db *gorm.DB) *AuditService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &AuditService{
		Log:  logger,
		Repo: repository.NewAuditRepository(ctx, db),
	}
}

func (s *AuditService) GetAll(page, limit int) ([]dto.AuditEntryResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	entries, total, err := s.Repo.FindAll(limit, offset)
	if err != nil {
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch audit log"}
	}

	var result []dto.AuditEntryResponse
	for _, e := range entries {
		result = append(result, dto.AuditEntryResponse{
			ID:        e.ID,
			Action:    e.Action,
			UserID:    e.UserID,
			UserName:  e.UserName,
			Details:   e.Details,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, total, nil
}

func (s *AuditService) CreateEntry(req dto.CreateAuditEntryRequest) *dto.ApiError {
	entry := &entity.AuditEntry{
		ID:       uuid.New().String(),
		Action:   req.Action,
		UserID:   req.UserID,
		UserName: req.UserName,
		Details:  req.Details,
	}

	if err := s.Repo.Create(entry); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create audit entry")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to log audit entry"}
	}
	return nil
}
