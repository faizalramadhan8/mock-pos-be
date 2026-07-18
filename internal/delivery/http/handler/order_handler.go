package handler

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type OrderController struct {
	Log     *zerolog.Logger
	Service *usecase.OrderService
}

func NewOrderController(ctx context.Context) *OrderController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &OrderController{
		Log:     logger,
		Service: usecase.NewOrderService(ctx, db),
	}
}

// GetAll — enhanced dengan Path B pagination (Bu Santi 19 Jul 2026):
//   ?status=       filter status (all|completed|pending|cancelled|refunded)
//   ?from=YYYY-MM-DD?to=YYYY-MM-DD    date range filter (created_at)
//   ?cursor=<ISO>  keyset pagination — get orders WHERE created_at < cursor
//   ?limit=N       default 500 (max 2000)
//   ?search=       substring match customer name / phone / order id
//
// Response.Meta:
//   next_cursor: string — pass ke request berikutnya untuk load more (empty = habis)
//   count: int — length dari body ini
//   total: int64 — total matching records (kalau tanpa cursor, else -1)
//
// Backward compat: existing FE call tanpa cursor/from/to tetap works,
// cuma dapet 500 recent (limit default). Path A compatibility.
func (ctrl *OrderController) GetAll(c *fiber.Ctx) error {
	status := c.Query("status", "")
	from := c.Query("from", "")
	to := c.Query("to", "")
	cursor := c.Query("cursor", "")
	search := c.Query("search", "")
	limit, _ := strconv.Atoi(c.Query("limit", "500"))
	if limit <= 0 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}

	orders, nextCursor, total, fail := ctrl.Service.GetAllWithCursor(status, from, to, cursor, search, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    orders,
		Meta: map[string]interface{}{
			"total":       total,
			"count":       len(orders),
			"limit":       limit,
			"next_cursor": nextCursor,
		},
	})
}

func (ctrl *OrderController) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	order, fail := ctrl.Service.GetByID(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: order})
}

func (ctrl *OrderController) Create(c *fiber.Ctx) error {
	var req dto.CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}

	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.Create(req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "successfully", Body: resp})
}

func (ctrl *OrderController) Cancel(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, fail := ctrl.Service.CancelOrder(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Order cancelled", Body: resp})
}

func (ctrl *OrderController) ResendWA(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	if fail := ctrl.Service.ResendReceiptWA(id, claims.ID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Receipt sent to WhatsApp"})
}

// CreatePending — create an order in pending state (customer orders online,
// kasir inputs, stok belum dipotong, WA invoice dikirim).
func (ctrl *OrderController) CreatePending(c *fiber.Ctx) error {
	var req dto.CreatePendingOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.CreatePending(req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "successfully", Body: resp})
}

// MarkAsPaid — flip pending order to completed with split payment info.
func (ctrl *OrderController) MarkAsPaid(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.MarkAsPaidRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.MarkAsPaid(id, req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Order marked as paid", Body: resp})
}

// EditPayments — admin/superadmin ubah metode pembayaran order completed.
// Skenario: kasir salah pilih Transfer padahal QRIS. Bu Santi 12 Jul 2026.
func (ctrl *OrderController) EditPayments(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.EditPaymentsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.EditPayments(id, req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Payment method updated", Body: resp})
}

// CancelPending — cancel a pending order (stok tidak disentuh).
func (ctrl *OrderController) CancelPending(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	if fail := ctrl.Service.CancelPending(id, claims.ID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Pending order cancelled"})
}

// ResendPendingInvoice — resend the WA invoice for a pending order.
func (ctrl *OrderController) ResendPendingInvoice(c *fiber.Ctx) error {
	id := c.Params("id")
	bankID := c.Query("bank_account_id", "")
	if fail := ctrl.Service.ResendPendingInvoiceWA(id, bankID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Invoice sent to customer"})
}

// Aggregate — server-side aggregation for /orders/aggregate.
// Replaces FE-side aggregation that broke when orders > fetch limit.
func (ctrl *OrderController) Aggregate(c *fiber.Ctx) error {
	from := c.Query("from", "")
	to := c.Query("to", "")
	resp, fail := ctrl.Service.Aggregate(from, to)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *OrderController) GetStats(c *fiber.Ctx) error {
	revenue, count, fail := ctrl.Service.GetRevenueStats()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body: map[string]interface{}{
			"revenue":     revenue,
			"order_count": count,
		},
	})
}
