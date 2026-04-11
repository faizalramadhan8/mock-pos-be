package usecase

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type SettingsService struct {
	Log  *zerolog.Logger
	Repo *repository.SettingsRepository
}

func NewSettingsService(ctx context.Context, db *gorm.DB) *SettingsService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &SettingsService{
		Log:  logger,
		Repo: repository.NewSettingsRepository(ctx, db),
	}
}

func (s *SettingsService) Get() (*dto.SettingsResponse, *dto.ApiError) {
	settings, err := s.Repo.Get()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Settings not found"}
	}
	return s.toResponse(settings), nil
}

func (s *SettingsService) Update(req dto.UpdateSettingsRequest) (*dto.SettingsResponse, *dto.ApiError) {
	settings, err := s.Repo.Get()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Settings not found"}
	}

	if req.StoreName != "" {
		settings.StoreName = req.StoreName
	}
	if req.StoreAddress != "" {
		settings.StoreAddress = req.StoreAddress
	}
	if req.StorePhone != "" {
		settings.StorePhone = req.StorePhone
	}
	if req.PPNRate >= 0 {
		settings.PPNRate = req.PPNRate
	}
	if req.LabelWidth > 0 {
		settings.LabelWidth = req.LabelWidth
	}
	if req.LabelHeight > 0 {
		settings.LabelHeight = req.LabelHeight
	}

	if err := s.Repo.Update(settings); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update settings")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update settings"}
	}

	return s.toResponse(settings), nil
}

func (s *SettingsService) AddBankAccount(req dto.AddBankAccountRequest) (*dto.SettingsResponse, *dto.ApiError) {
	settings, err := s.Repo.Get()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Settings not found"}
	}

	account := &entity.BankAccount{
		ID:            uuid.New().String(),
		SettingsID:    settings.ID,
		BankName:      req.BankName,
		AccountNumber: req.AccountNumber,
		AccountHolder: req.AccountHolder,
	}

	if err := s.Repo.AddBankAccount(account); err != nil {
		s.Log.Error().Err(err).Msg("Failed to add bank account")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to add bank account"}
	}

	// Reload settings
	settings, _ = s.Repo.Get()
	return s.toResponse(settings), nil
}

func (s *SettingsService) DeleteBankAccount(id string) (*dto.SettingsResponse, *dto.ApiError) {
	if err := s.Repo.DeleteBankAccount(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete bank account")
		return &dto.SettingsResponse{}, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete bank account"}
	}

	settings, _ := s.Repo.Get()
	return s.toResponse(settings), nil
}

func (s *SettingsService) toResponse(settings *entity.Settings) *dto.SettingsResponse {
	resp := &dto.SettingsResponse{
		ID:           settings.ID,
		StoreName:    settings.StoreName,
		StoreAddress: settings.StoreAddress,
		StorePhone:   settings.StorePhone,
		PPNRate:      settings.PPNRate,
		LabelWidth:   settings.LabelWidth,
		LabelHeight:  settings.LabelHeight,
	}

	for _, ba := range settings.BankAccounts {
		resp.BankAccounts = append(resp.BankAccounts, dto.BankAccountResponse{
			ID:            ba.ID,
			BankName:      ba.BankName,
			AccountNumber: ba.AccountNumber,
			AccountHolder: ba.AccountHolder,
		})
	}
	return resp
}
