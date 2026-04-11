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
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type CashSessionService struct {
	Log  *zerolog.Logger
	Repo *repository.CashSessionRepository
}

func NewCashSessionService(ctx context.Context, db *gorm.DB) *CashSessionService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &CashSessionService{
		Log:  logger,
		Repo: repository.NewCashSessionRepository(ctx, db),
	}
}

func (s *CashSessionService) GetAll() ([]dto.CashSessionResponse, *dto.ApiError) {
	sessions, err := s.Repo.FindAll()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch sessions"}
	}

	var result []dto.CashSessionResponse
	for _, sess := range sessions {
		result = append(result, s.toResponse(&sess))
	}
	return result, nil
}

func (s *CashSessionService) GetOpen() (*dto.CashSessionResponse, *dto.ApiError) {
	session, err := s.Repo.FindOpenSession()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "No open session"}
	}
	resp := s.toResponse(session)
	return &resp, nil
}

func (s *CashSessionService) Open(req dto.OpenCashSessionRequest) (*dto.CashSessionResponse, *dto.ApiError) {
	session := &entity.CashSession{
		ID:          uuid.New().String(),
		Date:        util.ParseDateOnly(req.Date),
		OpeningCash: req.OpeningCash,
		OpenedBy:    req.OpenedBy,
		OpenedAt:    time.Now(),
	}

	if err := s.Repo.Create(session); err != nil {
		s.Log.Error().Err(err).Msg("Failed to open cash session")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to open session"}
	}

	resp := s.toResponse(session)
	return &resp, nil
}

func (s *CashSessionService) Close(id string, req dto.CloseCashSessionRequest) (*dto.CashSessionResponse, *dto.ApiError) {
	session, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Session not found"}
	}

	now := time.Now()
	if err := s.Repo.Close(id, map[string]interface{}{
		"expected_cash": req.ExpectedCash,
		"actual_cash":   req.ActualCash,
		"difference":    req.Difference,
		"notes":         req.Notes,
		"closed_by":     req.ClosedBy,
		"closed_at":     now,
	}); err != nil {
		s.Log.Error().Err(err).Msg("Failed to close cash session")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to close session"}
	}

	session.ExpectedCash = req.ExpectedCash
	session.ActualCash = req.ActualCash
	session.Difference = req.Difference
	session.Notes = req.Notes
	session.ClosedBy = req.ClosedBy
	session.ClosedAt = &now

	resp := s.toResponse(session)
	return &resp, nil
}

func (s *CashSessionService) toResponse(sess *entity.CashSession) dto.CashSessionResponse {
	resp := dto.CashSessionResponse{
		ID:           sess.ID,
		Date:         sess.Date,
		OpeningCash:  sess.OpeningCash,
		OpenedBy:     sess.OpenedBy,
		OpenedAt:     sess.OpenedAt.Format(time.RFC3339),
		ExpectedCash: sess.ExpectedCash,
		ActualCash:   sess.ActualCash,
		Difference:   sess.Difference,
		Notes:        sess.Notes,
		ClosedBy:     sess.ClosedBy,
	}
	if sess.ClosedAt != nil {
		closedAt := sess.ClosedAt.Format(time.RFC3339)
		resp.ClosedAt = &closedAt
	}
	return resp
}
