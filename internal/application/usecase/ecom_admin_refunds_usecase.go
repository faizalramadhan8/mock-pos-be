package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminRefundsService — Sprint 4 Chunk 2 (31 Jul 2026).
// Manual refund flow: Bu Santi transfer out-of-band → catat di sistem +
// pilih item untuk restock. Update stock_ecom secara atomic.
type EcomAdminRefundsService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAdminRefundsService(ctx context.Context, db *gorm.DB) *EcomAdminRefundsService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminRefundsService{DB: db, Log: logger}
}

// Create — record refund + restock items dalam 1 transaction.
// Atomic: kalau gagal update stock, rollback insert refund.
func (s *EcomAdminRefundsService) Create(adminID string, req dto.EcomRefundCreateRequest) (*dto.EcomRefundResponse, *dto.ApiError) {
	// Verify order exists + is ecom order.
	var order entity.Order
	if err := s.DB.Where("id = ? AND order_source = 'ecom' AND deleted_at IS NULL", req.OrderID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Pesanan tidak ditemukan"}
	}
	if req.Amount > order.Total {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Amount refund tidak boleh > total pesanan",
		}
	}

	refundID := uuid.New().String()
	var restockedJSON *string
	if len(req.RestockItems) > 0 {
		b, _ := json.Marshal(req.RestockItems)
		s := string(b)
		restockedJSON = &s
	}

	refund := entity.EcomRefund{
		ID:             refundID,
		OrderID:        req.OrderID,
		Amount:         req.Amount,
		Method:         req.Method,
		RefundedBy:     adminID,
		RefundedAt:     time.Now(),
		RestockedItems: restockedJSON,
	}
	if req.ComplaintID != "" {
		refund.ComplaintID = &req.ComplaintID
	}
	if req.Note != "" {
		refund.Note = &req.Note
	}

	// Atomic tx: insert refund + restock stock_ecom + auto-resolve complaint (opsional).
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&refund).Error; e != nil {
			return e
		}
		// Restock items — atomic increment via gorm.Expr (rule stock atomic).
		for _, item := range req.RestockItems {
			if e := tx.Model(&entity.Product{}).
				Where("id = ?", item.ProductID).
				Update("stock_ecom", gorm.Expr("stock_ecom + ?", item.Qty)).Error; e != nil {
				return e
			}
		}
		// Kalau linked ke complaint, auto-set status=resolved + resolved_at.
		if req.ComplaintID != "" {
			tx.Model(&entity.EcomComplaint{}).
				Where("id = ? AND status IN ('open', 'in_review')", req.ComplaintID).
				Updates(map[string]interface{}{
					"status":      "resolved",
					"resolved_at": time.Now(),
					"admin_id":    adminID,
				})
		}
		return nil
	})
	if err != nil {
		s.Log.Error().Err(err).Str("order_id", req.OrderID).Msg("Failed to create refund")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan refund"}
	}

	return s.toResponse(&refund), nil
}

// ListByOrder — daftar semua refund event untuk 1 order (partial refund kasus).
func (s *EcomAdminRefundsService) ListByOrder(orderID string) ([]dto.EcomRefundResponse, *dto.ApiError) {
	var refunds []entity.EcomRefund
	if err := s.DB.Where("order_id = ?", orderID).
		Order("refunded_at DESC").
		Find(&refunds).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil daftar refund"}
	}
	// Batch fetch admin names (kecil, in-memory).
	names := map[string]string{}
	if len(refunds) > 0 {
		ids := make([]string, 0, len(refunds))
		for _, r := range refunds {
			ids = append(ids, r.RefundedBy)
		}
		var users []entity.User
		s.DB.Select("id, fullname").Where("id IN ?", ids).Find(&users)
		for _, u := range users {
			names[u.ID] = u.FullName
		}
	}
	out := make([]dto.EcomRefundResponse, 0, len(refunds))
	for i := range refunds {
		resp := s.toResponse(&refunds[i])
		resp.RefundedByName = names[refunds[i].RefundedBy]
		out = append(out, *resp)
	}
	return out, nil
}

func (s *EcomAdminRefundsService) toResponse(r *entity.EcomRefund) *dto.EcomRefundResponse {
	out := &dto.EcomRefundResponse{
		ID:         r.ID,
		OrderID:    r.OrderID,
		Amount:     r.Amount,
		Method:     r.Method,
		RefundedBy: r.RefundedBy,
		RefundedAt: r.RefundedAt.Format(time.RFC3339),
	}
	if r.ComplaintID != nil {
		out.ComplaintID = *r.ComplaintID
	}
	if r.Note != nil {
		out.Note = *r.Note
	}
	if r.RestockedItems != nil {
		var items []dto.RefundRestockItem
		if json.Unmarshal([]byte(*r.RestockedItems), &items) == nil {
			out.RestockedItems = items
		}
	}
	return out
}
