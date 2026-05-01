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
	Log         *zerolog.Logger
	Repo        *repository.ProductRepository
	HistoryRepo *repository.ProductPriceHistoryRepository
}

func NewProductService(ctx context.Context, db *gorm.DB) *ProductService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &ProductService{
		Log:         logger,
		Repo:        repository.NewProductRepository(ctx, db),
		HistoryRepo: repository.NewProductPriceHistoryRepository(ctx, db),
	}
}

// logPriceChange closes any active row of the given (product, type) and
// inserts a fresh active row at `now`. Errors are logged but never block the
// caller — price-history is an audit trail, not a transactional dependency.
func (s *ProductService) logPriceChange(productID, priceType string, price float64, changedBy *string, note string) {
	now := time.Now()
	if err := s.HistoryRepo.CloseActive(productID, priceType, now); err != nil {
		s.Log.Warn().Err(err).Str("product_id", productID).Str("type", priceType).Msg("price history: close active failed")
	}
	row := &entity.ProductPriceHistory{
		ID:        uuid.New().String(),
		ProductID: productID,
		PriceType: priceType,
		Price:     price,
		Status:    "active",
		StartDate: now,
		ChangedBy: changedBy,
		Note:      note,
	}
	if err := s.HistoryRepo.Create(row); err != nil {
		s.Log.Warn().Err(err).Str("product_id", productID).Str("type", priceType).Msg("price history: insert failed")
	}
}

func (s *ProductService) GetAll(search, categoryID, supplierID string, page, limit int) ([]dto.ProductResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	products, total, err := s.Repo.FindAll(search, categoryID, supplierID, limit, offset)
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

func (s *ProductService) Create(req dto.CreateProductRequest, userID string) (*dto.ProductResponse, *dto.ApiError) {
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
		SupplierID:    req.SupplierID,
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

	// Seed price history with the initial regular/purchase/member rows so
	// subsequent edits already have a closed predecessor to compare against.
	var changer *string
	if userID != "" {
		changer = &userID
	}
	s.logPriceChange(product.ID, "regular", product.SellingPrice, changer, "initial")
	s.logPriceChange(product.ID, "purchase", product.PurchasePrice, changer, "initial")
	if product.MemberPrice != nil && *product.MemberPrice > 0 {
		s.logPriceChange(product.ID, "member", *product.MemberPrice, changer, "initial")
	}

	p, _ := s.Repo.FindByID(product.ID)
	if p != nil {
		product = p
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) Update(id string, req dto.UpdateProductRequest, userID string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}

	// Snapshot pre-update prices so we know what actually changed.
	prevPurchase := product.PurchasePrice
	prevSelling := product.SellingPrice
	var prevMember *float64
	if product.MemberPrice != nil {
		v := *product.MemberPrice
		prevMember = &v
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
	// SupplierID: pointer — explicit null clears it, value sets it
	if req.SupplierID != nil {
		if *req.SupplierID == "" {
			product.SupplierID = nil
		} else {
			product.SupplierID = req.SupplierID
		}
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

	// Log price changes (only when value actually moved). Audit trail
	// is best-effort: failures are logged inside logPriceChange, never block.
	var changer *string
	if userID != "" {
		changer = &userID
	}
	if product.PurchasePrice != prevPurchase {
		s.logPriceChange(product.ID, "purchase", product.PurchasePrice, changer, "")
	}
	if product.SellingPrice != prevSelling {
		s.logPriceChange(product.ID, "regular", product.SellingPrice, changer, "")
	}
	memberChanged := false
	switch {
	case prevMember == nil && product.MemberPrice != nil:
		memberChanged = true
	case prevMember != nil && product.MemberPrice == nil:
		memberChanged = true
	case prevMember != nil && product.MemberPrice != nil && *prevMember != *product.MemberPrice:
		memberChanged = true
	}
	if memberChanged {
		var newMember float64
		if product.MemberPrice != nil {
			newMember = *product.MemberPrice
		}
		s.logPriceChange(product.ID, "member", newMember, changer, "")
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

// Delete soft-deletes a product. Previous orders that reference it keep their
// own name/price snapshots in order_items, so history stays intact.
func (s *ProductService) Delete(id string) *dto.ApiError {
	if _, err := s.Repo.FindByID(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete product")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete product"}
	}
	return nil
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

// GetPriceHistory returns chronological price changes for a product. Optional
// priceType filter — empty = all (regular + member + purchase).
func (s *ProductService) GetPriceHistory(productID, priceType string) ([]dto.ProductPriceHistoryResponse, *dto.ApiError) {
	if _, err := s.Repo.FindByID(productID); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	rows, err := s.HistoryRepo.FindByProduct(productID, priceType)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch price history")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch price history"}
	}
	out := make([]dto.ProductPriceHistoryResponse, 0, len(rows))
	for _, r := range rows {
		row := dto.ProductPriceHistoryResponse{
			ID:        r.ID,
			ProductID: r.ProductID,
			PriceType: r.PriceType,
			Price:     r.Price,
			Status:    r.Status,
			StartDate: r.StartDate.Format(time.RFC3339),
			ChangedBy: r.ChangedBy,
			Note:      r.Note,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		}
		if r.EndDate != nil {
			s := r.EndDate.Format(time.RFC3339)
			row.EndDate = &s
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *ProductService) toResponse(p *entity.Product) dto.ProductResponse {
	resp := dto.ProductResponse{
		ID:            p.ID,
		SKU:           p.SKU,
		Barcode:       p.Barcode,
		Name:          p.Name,
		NameID:        p.NameID,
		CategoryID:    p.CategoryID,
		SupplierID:    p.SupplierID,
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
	if p.Supplier != nil {
		sup := dto.SupplierResponse{
			ID:        p.Supplier.ID,
			Name:      p.Supplier.Name,
			Phone:     p.Supplier.Phone,
			Email:     p.Supplier.Email,
			Address:   p.Supplier.Address,
			CreatedAt: p.Supplier.CreatedAt.Format(time.RFC3339),
		}
		resp.Supplier = &sup
	}
	return resp
}
