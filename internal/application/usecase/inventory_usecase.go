package usecase

import (
	"context"
	"fmt"
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

type InventoryService struct {
	Log         *zerolog.Logger
	DB          *gorm.DB
	MoveRepo    *repository.StockMovementRepository
	BatchRepo   *repository.StockBatchRepository
	ProductRepo *repository.ProductRepository
}

func NewInventoryService(ctx context.Context, db *gorm.DB) *InventoryService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &InventoryService{
		Log:         logger,
		DB:          db,
		MoveRepo:    repository.NewStockMovementRepository(ctx, db),
		BatchRepo:   repository.NewStockBatchRepository(ctx, db),
		ProductRepo: repository.NewProductRepository(ctx, db),
	}
}

func (s *InventoryService) GetAllMovements(movementType string, page, limit int) ([]dto.StockMovementResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	movements, total, err := s.MoveRepo.FindAll(movementType, limit, offset)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch stock movements")
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch movements"}
	}

	var result []dto.StockMovementResponse
	for _, m := range movements {
		result = append(result, s.toMoveResponse(&m))
	}
	return result, total, nil
}

func (s *InventoryService) CreateMovement(req dto.CreateStockMovementRequest, userID string) (*dto.StockMovementResponse, *dto.ApiError) {
	tx := s.DB.Begin()

	movement := &entity.StockMovement{
		ID:            uuid.New().String(),
		ProductID:     req.ProductID,
		Type:          req.Type,
		Quantity:      req.Quantity,
		UnitType:      req.UnitType,
		UnitPrice:     req.UnitPrice,
		Note:          req.Note,
		PaymentTerms:  req.PaymentTerms,
		PaymentStatus: req.PaymentStatus,
		CreatedBy:     userID,
	}

	if movement.UnitType == "" {
		movement.UnitType = "individual"
	}
	if movement.PaymentStatus == "" {
		movement.PaymentStatus = "paid"
	}

	if req.ExpiryDate != "" {
		parsed := util.ParseDateOnly(req.ExpiryDate)
		movement.ExpiryDate = &parsed
	}
	if req.SupplierID != "" {
		movement.SupplierID = &req.SupplierID
	}
	if req.DueDate != "" {
		parsed := util.ParseDateOnly(req.DueDate)
		movement.DueDate = &parsed
	}

	// Calculate stock delta
	product, err := s.ProductRepo.FindByID(req.ProductID)
	if err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}

	stockDelta := req.Quantity
	if req.UnitType == "box" && product.QtyPerBox > 0 {
		stockDelta = req.Quantity * product.QtyPerBox
	}

	if req.Type == "in" {
		tx.Model(&entity.Product{}).Where("id = ?", req.ProductID).
			Update("stock", gorm.Expr("stock + ?", stockDelta))

		// Create batch for stock-in
		batchNumber := req.BatchNumber
		if batchNumber == "" {
			batchNumber = fmt.Sprintf("B-%s-%03d", time.Now().Format("20060102"), time.Now().UnixMilli()%1000)
		}
		batch := &entity.StockBatch{
			ID:          uuid.New().String(),
			ProductID:   req.ProductID,
			Quantity:    stockDelta,
			Note:        req.Note,
			BatchNumber: batchNumber,
		}
		if req.ExpiryDate != "" {
			parsed := util.ParseDateOnly(req.ExpiryDate)
			batch.ExpiryDate = &parsed
		}
		tx.Create(batch)
	} else {
		if product.Stock < stockDelta {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Insufficient stock"}
		}
		tx.Model(&entity.Product{}).Where("id = ?", req.ProductID).
			Update("stock", gorm.Expr("stock - ?", stockDelta))

		// Consume FIFO batches for stock-out
		s.consumeFIFO(tx, req.ProductID, stockDelta)
	}

	if err := tx.Create(movement).Error; err != nil {
		tx.Rollback()
		s.Log.Error().Err(err).Msg("Failed to create stock movement")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create movement"}
	}

	tx.Commit()
	resp := s.toMoveResponse(movement)
	return &resp, nil
}

func (s *InventoryService) UpdatePaymentStatus(id string, status string) (*dto.StockMovementResponse, *dto.ApiError) {
	movement, err := s.MoveRepo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Movement not found"}
	}

	movement.PaymentStatus = status
	if err := s.MoveRepo.Update(movement); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update payment status"}
	}

	resp := s.toMoveResponse(movement)
	return &resp, nil
}

func (s *InventoryService) GetAllBatches(page, limit int) ([]dto.StockBatchResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	batches, total, err := s.BatchRepo.FindAll(limit, offset)
	if err != nil {
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch batches"}
	}

	var result []dto.StockBatchResponse
	for _, b := range batches {
		result = append(result, s.toBatchResponse(&b))
	}
	return result, total, nil
}

func (s *InventoryService) GetExpiringBatches(withinDays int) ([]dto.StockBatchResponse, *dto.ApiError) {
	if withinDays <= 0 {
		withinDays = 60
	}
	batches, err := s.BatchRepo.FindExpiring(withinDays)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch expiring batches"}
	}

	var result []dto.StockBatchResponse
	for _, b := range batches {
		result = append(result, s.toBatchResponse(&b))
	}
	return result, nil
}

func (s *InventoryService) ConsumeFIFO(productID string, qty int) *dto.ApiError {
	tx := s.DB.Begin()
	s.consumeFIFO(tx, productID, qty)
	tx.Commit()
	return nil
}

func (s *InventoryService) consumeFIFO(tx *gorm.DB, productID string, qty int) {
	var batches []entity.StockBatch
	tx.Where("product_id = ? AND quantity > 0", productID).Order("received_at ASC").Find(&batches)

	remaining := qty
	for i := range batches {
		if remaining <= 0 {
			break
		}
		if batches[i].Quantity >= remaining {
			batches[i].Quantity -= remaining
			remaining = 0
		} else {
			remaining -= batches[i].Quantity
			batches[i].Quantity = 0
		}
		tx.Save(&batches[i])
	}
}

func (s *InventoryService) toMoveResponse(m *entity.StockMovement) dto.StockMovementResponse {
	resp := dto.StockMovementResponse{
		ID:            m.ID,
		ProductID:     m.ProductID,
		Type:          m.Type,
		Quantity:      m.Quantity,
		UnitType:      m.UnitType,
		UnitPrice:     m.UnitPrice,
		Note:          m.Note,
		ExpiryDate:    m.ExpiryDate,
		SupplierID:    m.SupplierID,
		PaymentTerms:  m.PaymentTerms,
		DueDate:       m.DueDate,
		PaymentStatus: m.PaymentStatus,
		CreatedBy:     m.CreatedBy,
		CreatedAt:     m.CreatedAt.Format(time.RFC3339),
	}
	return resp
}

func (s *InventoryService) toBatchResponse(b *entity.StockBatch) dto.StockBatchResponse {
	resp := dto.StockBatchResponse{
		ID:          b.ID,
		ProductID:   b.ProductID,
		Quantity:    b.Quantity,
		ExpiryDate:  b.ExpiryDate,
		ReceivedAt:  b.ReceivedAt.Format(time.RFC3339),
		Note:        b.Note,
		BatchNumber: b.BatchNumber,
	}
	if b.Product != nil {
		pr := dto.ProductResponse{
			ID:   b.Product.ID,
			SKU:  b.Product.SKU,
			Name: b.Product.Name,
		}
		resp.Product = &pr
	}
	return resp
}
