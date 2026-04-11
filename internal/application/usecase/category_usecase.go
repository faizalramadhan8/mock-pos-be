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

type CategoryService struct {
	Log  *zerolog.Logger
	Repo *repository.CategoryRepository
}

func NewCategoryService(ctx context.Context, db *gorm.DB) *CategoryService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &CategoryService{
		Log:  logger,
		Repo: repository.NewCategoryRepository(ctx, db),
	}
}

func (s *CategoryService) GetAll() ([]dto.CategoryResponse, *dto.ApiError) {
	categories, err := s.Repo.FindAll()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch categories")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch categories"}
	}

	var result []dto.CategoryResponse
	for _, c := range categories {
		result = append(result, dto.CategoryResponse{
			ID:     c.ID,
			Name:   c.Name,
			NameID: c.NameID,
			Icon:   c.Icon,
			Color:  c.Color,
		})
	}
	return result, nil
}

func (s *CategoryService) Create(req dto.CreateCategoryRequest) (*dto.CategoryResponse, *dto.ApiError) {
	cat := &entity.Category{
		ID:     uuid.New().String(),
		Name:   req.Name,
		NameID: req.NameID,
		Icon:   req.Icon,
		Color:  req.Color,
	}

	if err := s.Repo.Create(cat); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create category")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create category"}
	}

	return &dto.CategoryResponse{
		ID:     cat.ID,
		Name:   cat.Name,
		NameID: cat.NameID,
		Icon:   cat.Icon,
		Color:  cat.Color,
	}, nil
}

func (s *CategoryService) Update(id string, req dto.UpdateCategoryRequest) (*dto.CategoryResponse, *dto.ApiError) {
	cat, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Category not found"}
	}

	if req.Name != "" {
		cat.Name = req.Name
	}
	if req.NameID != "" {
		cat.NameID = req.NameID
	}
	if req.Icon != "" {
		cat.Icon = req.Icon
	}
	if req.Color != "" {
		cat.Color = req.Color
	}

	if err := s.Repo.Update(cat); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update category")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update category"}
	}

	return &dto.CategoryResponse{
		ID:     cat.ID,
		Name:   cat.Name,
		NameID: cat.NameID,
		Icon:   cat.Icon,
		Color:  cat.Color,
	}, nil
}

func (s *CategoryService) Delete(id string) *dto.ApiError {
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete category")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete category"}
	}
	return nil
}
