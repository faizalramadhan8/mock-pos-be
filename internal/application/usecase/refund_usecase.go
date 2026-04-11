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

type RefundService struct {
	Log         *zerolog.Logger
	DB          *gorm.DB
	Repo        *repository.RefundRepository
	OrderRepo   *repository.OrderRepository
	ProductRepo *repository.ProductRepository
}

func NewRefundService(ctx context.Context, db *gorm.DB) *RefundService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &RefundService{
		Log:         logger,
		DB:          db,
		Repo:        repository.NewRefundRepository(ctx, db),
		OrderRepo:   repository.NewOrderRepository(ctx, db),
		ProductRepo: repository.NewProductRepository(ctx, db),
	}
}

func (s *RefundService) Create(req dto.CreateRefundRequest, userID string) (*dto.RefundResponse, *dto.ApiError) {
	order, err := s.OrderRepo.FindByID(req.OrderID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}

	tx := s.DB.Begin()

	refund := &entity.Refund{
		ID:        uuid.New().String(),
		OrderID:   req.OrderID,
		Amount:    req.Amount,
		Reason:    req.Reason,
		CreatedBy: userID,
	}

	for _, item := range req.Items {
		refundItem := entity.RefundItem{
			ID:           uuid.New().String(),
			RefundID:     refund.ID,
			ProductID:    item.ProductID,
			Name:         item.Name,
			Quantity:     item.Quantity,
			UnitType:     item.UnitType,
			UnitPrice:    item.UnitPrice,
			RefundAmount: item.RefundAmount,
		}
		if refundItem.UnitType == "" {
			refundItem.UnitType = "individual"
		}
		refund.Items = append(refund.Items, refundItem)

		// Restore stock
		stockDelta := item.Quantity
		if item.UnitType == "box" {
			product, err := s.ProductRepo.FindByID(item.ProductID)
			if err == nil && product.QtyPerBox > 0 {
				stockDelta = item.Quantity * product.QtyPerBox
			}
		}
		tx.Model(&entity.Product{}).Where("id = ?", item.ProductID).
			Update("stock", gorm.Expr("stock + ?", stockDelta))
	}

	if err := tx.Create(refund).Error; err != nil {
		tx.Rollback()
		s.Log.Error().Err(err).Msg("Failed to create refund")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create refund"}
	}

	// Mark order as refunded
	order.Status = "refunded"
	tx.Save(order)

	tx.Commit()
	resp := s.toResponse(refund)
	return &resp, nil
}

func (s *RefundService) GetByOrderID(orderID string) ([]dto.RefundResponse, *dto.ApiError) {
	refunds, err := s.Repo.FindByOrderID(orderID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch refunds"}
	}

	var result []dto.RefundResponse
	for _, r := range refunds {
		result = append(result, s.toResponse(&r))
	}
	return result, nil
}

func (s *RefundService) toResponse(r *entity.Refund) dto.RefundResponse {
	resp := dto.RefundResponse{
		ID:        r.ID,
		OrderID:   r.OrderID,
		Amount:    r.Amount,
		Reason:    r.Reason,
		CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}

	for _, item := range r.Items {
		resp.Items = append(resp.Items, dto.RefundItemResponse{
			ID:           item.ID,
			ProductID:    item.ProductID,
			Name:         item.Name,
			Quantity:     item.Quantity,
			UnitType:     item.UnitType,
			UnitPrice:    item.UnitPrice,
			RefundAmount: item.RefundAmount,
		})
	}
	return resp
}
