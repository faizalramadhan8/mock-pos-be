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

type InventoryController struct {
	Log     *zerolog.Logger
	Service *usecase.InventoryService
}

func NewInventoryController(ctx context.Context) *InventoryController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &InventoryController{
		Log:     logger,
		Service: usecase.NewInventoryService(ctx, db),
	}
}

func (ctrl *InventoryController) GetAllMovements(c *fiber.Ctx) error {
	movementType := c.Query("type", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	movements, total, fail := ctrl.Service.GetAllMovements(movementType, page, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    movements,
		Meta: map[string]interface{}{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (ctrl *InventoryController) CreateMovement(c *fiber.Ctx) error {
	var req dto.CreateStockMovementRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}

	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.CreateMovement(req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "successfully", Body: resp})
}

func (ctrl *InventoryController) UpdatePaymentStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdatePaymentStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}

	resp, fail := ctrl.Service.UpdatePaymentStatus(id, req.PaymentStatus)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *InventoryController) GetAllBatches(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	batches, total, fail := ctrl.Service.GetAllBatches(page, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    batches,
		Meta: map[string]interface{}{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (ctrl *InventoryController) GetExpiringBatches(c *fiber.Ctx) error {
	days, _ := strconv.Atoi(c.Query("days", "60"))
	batches, fail := ctrl.Service.GetExpiringBatches(days)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: batches})
}

func (ctrl *InventoryController) ConsumeFIFO(c *fiber.Ctx) error {
	var req dto.ConsumeFIFORequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}

	if fail := ctrl.Service.ConsumeFIFO(req.ProductID, req.Quantity); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "FIFO consumed successfully"})
}
