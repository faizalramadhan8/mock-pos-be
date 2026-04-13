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

type ProductService struct {
	Log  *zerolog.Logger
	Repo *repository.ProductRepository
}

func NewProductService(ctx context.Context, db *gorm.DB) *ProductService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &ProductService{
		Log:  logger,
		Repo: repository.NewProductRepository(ctx, db),
	}
}

func (s *ProductService) GetAll(search, categoryID string, page, limit int) ([]dto.ProductResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	products, total, err := s.Repo.FindAll(search, categoryID, limit, offset)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch products")
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch products"}
	}

	var result []dto.ProductResponse
	for _, p := range products {
		result = append(result, s.toResponse(&p))
	}
	return result, total, nil
}

func (s *ProductService) GetByID(id string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) GetBySKU(sku string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindBySKU(sku)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) GetLowStock() ([]dto.ProductResponse, *dto.ApiError) {
	products, err := s.Repo.FindLowStock()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch low stock products")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch low stock products"}
	}

	var result []dto.ProductResponse
	for _, p := range products {
		result = append(result, s.toResponse(&p))
	}
	return result, nil
}

func (s *ProductService) Create(req dto.CreateProductRequest) (*dto.ProductResponse, *dto.ApiError) {
	exists, err := s.Repo.ExistsBySKU(req.SKU)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to check SKU"}
	}
	if exists {
		return nil, &dto.ApiError{StatusCode: fiber.ErrConflict, Message: "SKU already exists"}
	}

	product := &entity.Product{
		ID:            uuid.New().String(),
		SKU:           req.SKU,
		Barcode:       req.Barcode,
		Name:          req.Name,
		NameID:        req.NameID,
		CategoryID:    req.CategoryID,
		PurchasePrice: req.PurchasePrice,
		SellingPrice:  req.SellingPrice,
		MemberPrice:   req.MemberPrice,
		QtyPerBox:     req.QtyPerBox,
		Stock:         req.Stock,
		Unit:          req.Unit,
		Image:         req.Image,
		MinStock:      req.MinStock,
		IsActive:      true,
	}

	if product.QtyPerBox == 0 {
		product.QtyPerBox = 1
	}

	if err := s.Repo.Create(product); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create product")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create product"}
	}

	p, _ := s.Repo.FindByID(product.ID)
	if p != nil {
		product = p
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) Update(id string, req dto.UpdateProductRequest) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}

	if req.Name != "" {
		product.Name = req.Name
	}
	if req.NameID != "" {
		product.NameID = req.NameID
	}
	if req.CategoryID != "" {
		product.CategoryID = req.CategoryID
	}
	if req.PurchasePrice > 0 {
		product.PurchasePrice = req.PurchasePrice
	}
	if req.SellingPrice > 0 {
		product.SellingPrice = req.SellingPrice
	}
	// MemberPrice: pointer — explicit null clears it, value sets it
	if req.MemberPrice != nil {
		if *req.MemberPrice <= 0 {
			product.MemberPrice = nil
		} else {
			product.MemberPrice = req.MemberPrice
		}
	}
	if req.QtyPerBox > 0 {
		product.QtyPerBox = req.QtyPerBox
	}
	if req.Unit != "" {
		product.Unit = req.Unit
	}
	if req.Image != "" {
		product.Image = req.Image
	}
	if req.MinStock >= 0 {
		product.MinStock = req.MinStock
	}
	if req.SKU != "" {
		product.SKU = req.SKU
	}
	if req.Barcode != "" {
		product.Barcode = req.Barcode
	}
	if req.Stock != nil {
		product.Stock = *req.Stock
	}

	if err := s.Repo.Update(product); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update product")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update product"}
	}

	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) AdjustStock(id string, delta int) (*dto.ProductResponse, *dto.ApiError) {
	if err := s.Repo.AdjustStock(id, delta); err != nil {
		s.Log.Error().Err(err).Msg("Failed to adjust stock")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to adjust stock"}
	}

	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) ToggleActive(id string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}

	product.IsActive = !product.IsActive
	if err := s.Repo.Update(product); err != nil {
		s.Log.Error().Err(err).Msg("Failed to toggle product active status")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update product"}
	}

	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) toResponse(p *entity.Product) dto.ProductResponse {
	resp := dto.ProductResponse{
		ID:            p.ID,
		SKU:           p.SKU,
		Barcode:       p.Barcode,
		Name:          p.Name,
		NameID:        p.NameID,
		CategoryID:    p.CategoryID,
		PurchasePrice: p.PurchasePrice,
		SellingPrice:  p.SellingPrice,
		MemberPrice:   p.MemberPrice,
		QtyPerBox:     p.QtyPerBox,
		Stock:         p.Stock,
		Unit:          p.Unit,
		Image:         p.Image,
		MinStock:      p.MinStock,
		IsActive:      p.IsActive,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
	}
	if p.Category != nil {
		cat := dto.CategoryResponse{
			ID:     p.Category.ID,
			Name:   p.Category.Name,
			NameID: p.Category.NameID,
			Icon:   p.Category.Icon,
			Color:  p.Category.Color,
		}
		resp.Category = &cat
	}
	return resp
}
