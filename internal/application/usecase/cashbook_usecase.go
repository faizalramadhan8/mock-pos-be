package usecase

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type CashbookService struct {
	Log  *zerolog.Logger
	Repo *repository.CashbookRepository
}

func NewCashbookService(ctx context.Context, db *gorm.DB) *CashbookService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &CashbookService{
		Log:  logger,
		Repo: repository.NewCashbookRepository(ctx, db),
	}
}

// GetOpeningBalance returns saldo awal periode. Empty (nil result) → tampil
// 0 di FE supaya "belum di-set" jelas vs "set ke 0 secara eksplisit".
func (s *CashbookService) GetOpeningBalance(year, month int) (*dto.CashbookOpeningBalanceResponse, *dto.ApiError) {
	ob, err := s.Repo.FindOpeningBalance(year, month)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch opening balance")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch opening balance"}
	}
	if ob == nil {
		return nil, nil
	}
	resp := toCashbookResponse(ob)
	return &resp, nil
}

func (s *CashbookService) SetOpeningBalance(req dto.SetOpeningBalanceRequest, userID string) (*dto.CashbookOpeningBalanceResponse, *dto.ApiError) {
	ob := &entity.CashbookOpeningBalance{
		ID:        uuid.New().String(),
		Year:      req.Year,
		Month:     req.Month,
		Balance:   req.Balance,
		Note:      req.Note,
		CreatedBy: userID,
	}
	if err := s.Repo.UpsertOpeningBalance(ob); err != nil {
		s.Log.Error().Err(err).Msg("Failed to upsert opening balance")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save opening balance"}
	}
	// Refetch supaya response konsisten (timestamp dll)
	refetched, _ := s.Repo.FindOpeningBalance(req.Year, req.Month)
	if refetched != nil {
		ob = refetched
	}
	resp := toCashbookResponse(ob)
	return &resp, nil
}

func (s *CashbookService) ListOpeningBalances() ([]dto.CashbookOpeningBalanceResponse, *dto.ApiError) {
	rows, err := s.Repo.FindAll()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to list opening balances"}
	}
	out := make([]dto.CashbookOpeningBalanceResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCashbookResponse(&r))
	}
	return out, nil
}

func toCashbookResponse(ob *entity.CashbookOpeningBalance) dto.CashbookOpeningBalanceResponse {
	return dto.CashbookOpeningBalanceResponse{
		ID:        ob.ID,
		Year:      ob.Year,
		Month:     ob.Month,
		Balance:   ob.Balance,
		Note:      ob.Note,
		CreatedBy: ob.CreatedBy,
		CreatedAt: ob.CreatedAt.Format(time.RFC3339),
		UpdatedAt: ob.UpdatedAt.Format(time.RFC3339),
	}
}
