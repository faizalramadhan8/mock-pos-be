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

type RedeemableItemService struct {
	Log  *zerolog.Logger
	DB   *gorm.DB
	Repo *repository.RedeemableItemRepository
}

func NewRedeemableItemService(ctx context.Context, db *gorm.DB) *RedeemableItemService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &RedeemableItemService{
		Log:  logger,
		DB:   db,
		Repo: repository.NewRedeemableItemRepository(ctx, db),
	}
}

func (s *RedeemableItemService) toResponse(it *entity.RedeemableItem) dto.RedeemableItemResponse {
	return dto.RedeemableItemResponse{
		ID:          it.ID,
		Name:        it.Name,
		Description: it.Description,
		Image:       it.Image,
		PointsCost:  it.PointsCost,
		Stock:       it.Stock,
		Redeemed:    it.Redeemed,
		IsActive:    it.IsActive,
		CreatedAt:   it.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   it.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *RedeemableItemService) List() ([]dto.RedeemableItemResponse, *dto.ApiError) {
	items, err := s.Repo.FindAll()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch redeemable items"}
	}
	out := make([]dto.RedeemableItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, s.toResponse(&it))
	}
	return out, nil
}

func (s *RedeemableItemService) ListActive() ([]dto.RedeemableItemResponse, *dto.ApiError) {
	items, err := s.Repo.FindActive()
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch redeemable items"}
	}
	out := make([]dto.RedeemableItemResponse, 0, len(items))
	for _, it := range items {
		out = append(out, s.toResponse(&it))
	}
	return out, nil
}

func (s *RedeemableItemService) Create(req dto.SaveRedeemableItemRequest) (*dto.RedeemableItemResponse, *dto.ApiError) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	item := &entity.RedeemableItem{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Image:       req.Image,
		PointsCost:  req.PointsCost,
		Stock:       req.Stock,
		IsActive:    isActive,
	}
	if err := s.Repo.Create(item); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create redeemable item")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create item"}
	}
	resp := s.toResponse(item)
	return &resp, nil
}

func (s *RedeemableItemService) Update(id string, req dto.SaveRedeemableItemRequest) (*dto.RedeemableItemResponse, *dto.ApiError) {
	item, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Item not found"}
	}
	item.Name = req.Name
	item.Description = req.Description
	item.Image = req.Image
	item.PointsCost = req.PointsCost
	item.Stock = req.Stock
	if req.IsActive != nil {
		item.IsActive = *req.IsActive
	}
	if err := s.Repo.Update(item); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update redeemable item")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update item"}
	}
	resp := s.toResponse(item)
	return &resp, nil
}

func (s *RedeemableItemService) Delete(id string) *dto.ApiError {
	if _, err := s.Repo.FindByID(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Item not found"}
	}
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete redeemable item")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete item"}
	}
	return nil
}
