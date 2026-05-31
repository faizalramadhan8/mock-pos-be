package handler

import (
	"context"
	"strconv"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type PurchaseInvoiceController struct {
	Log     *zerolog.Logger
	Service *usecase.PurchaseInvoiceService
}

func NewPurchaseInvoiceController(ctx context.Context) *PurchaseInvoiceController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &PurchaseInvoiceController{
		Log:     logger,
		Service: usecase.NewPurchaseInvoiceService(ctx, db),
	}
}

func (ctrl *PurchaseInvoiceController) GetAll(c *fiber.Ctx) error {
	status := c.Query("status", "")
	supplierID := c.Query("supplier_id", "")
	from := c.Query("from", "")
	to := c.Query("to", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	invoices, total, fail := ctrl.Service.GetAll(status, supplierID, from, to, page, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    invoices,
		Meta:    map[string]interface{}{"total": total, "page": page, "limit": limit},
	})
}

func (ctrl *PurchaseInvoiceController) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	inv, fail := ctrl.Service.GetByID(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: inv})
}

func (ctrl *PurchaseInvoiceController) Create(c *fiber.Ctx) error {
	var req dto.CreatePurchaseInvoiceRequest
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

func (ctrl *PurchaseInvoiceController) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.CreatePurchaseInvoiceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.Update(id, req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Invoice updated", Body: resp})
}

func (ctrl *PurchaseInvoiceController) MarkAsPaid(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, fail := ctrl.Service.MarkAsPaid(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Invoice ditandai lunas", Body: resp})
}

func (ctrl *PurchaseInvoiceController) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if fail := ctrl.Service.Delete(id); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Invoice dihapus"})
}
