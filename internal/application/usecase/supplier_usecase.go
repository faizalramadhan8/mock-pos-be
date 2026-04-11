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

type SupplierService struct {
	Log  *zerolog.Logger
	Repo *repository.SupplierRepository
}

func NewSupplierService(ctx context.Context, db *gorm.DB) *SupplierService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &SupplierService{
		Log:  logger,
		Repo: repository.NewSupplierRepository(ctx, db),
	}
}

func (s *SupplierService) GetAll(search string, page, limit int) ([]dto.SupplierResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	suppliers, total, err := s.Repo.FindAll(search, limit, offset)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch suppliers")
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch suppliers"}
	}

	var result []dto.SupplierResponse
	for _, sup := range suppliers {
		result = append(result, s.toResponse(&sup))
	}
	return result, total, nil
}

func (s *SupplierService) GetByID(id string) (*dto.SupplierResponse, *dto.ApiError) {
	sup, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Supplier not found"}
	}
	resp := s.toResponse(sup)
	return &resp, nil
}

func (s *SupplierService) Create(req dto.CreateSupplierRequest) (*dto.SupplierResponse, *dto.ApiError) {
	supplier := &entity.Supplier{
		ID:      uuid.New().String(),
		Name:    req.Name,
		Phone:   req.Phone,
		Email:   req.Email,
		Address: req.Address,
	}

	if err := s.Repo.Create(supplier); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create supplier")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create supplier"}
	}

	resp := s.toResponse(supplier)
	return &resp, nil
}

func (s *SupplierService) Update(id string, req dto.UpdateSupplierRequest) (*dto.SupplierResponse, *dto.ApiError) {
	supplier, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Supplier not found"}
	}

	if req.Name != "" {
		supplier.Name = req.Name
	}
	if req.Phone != "" {
		supplier.Phone = req.Phone
	}
	if req.Email != "" {
		supplier.Email = req.Email
	}
	if req.Address != "" {
		supplier.Address = req.Address
	}

	if err := s.Repo.Update(supplier); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update supplier")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update supplier"}
	}

	resp := s.toResponse(supplier)
	return &resp, nil
}

func (s *SupplierService) Delete(id string) *dto.ApiError {
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete supplier")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete supplier"}
	}
	return nil
}

func (s *SupplierService) toResponse(sup *entity.Supplier) dto.SupplierResponse {
	return dto.SupplierResponse{
		ID:        sup.ID,
		Name:      sup.Name,
		Phone:     sup.Phone,
		Email:     sup.Email,
		Address:   sup.Address,
		CreatedAt: sup.CreatedAt.Format(time.RFC3339),
	}
}
