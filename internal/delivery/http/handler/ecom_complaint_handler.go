package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomComplaintController struct {
	Log     *zerolog.Logger
	Service *usecase.EcomComplaintService
}

func NewEcomComplaintController(ctx context.Context) *EcomComplaintController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomComplaintController{
		Log:     logger,
		Service: usecase.NewEcomComplaintService(ctx, db),
	}
}

// Submit — customer submit komplain baru.
func (ctrl *EcomComplaintController) Submit(c *fiber.Ctx) error {
	var req dto.ComplaintSubmitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body"})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: "Invalid input", Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.Submit(claims.ID, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Komplain terkirim", Body: resp})
}

// ListForUser — customer lihat komplain miliknya.
func (ctrl *EcomComplaintController) ListForUser(c *fiber.Ctx) error {
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.ListForUser(claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// ListForAdmin — admin lihat semua komplain, filter by status.
func (ctrl *EcomComplaintController) ListForAdmin(c *fiber.Ctx) error {
	status := c.Query("status")
	resp, fail := ctrl.Service.ListForAdmin(status)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// Reply — admin reply komplain + update status.
func (ctrl *EcomComplaintController) Reply(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.ComplaintAdminReplyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body"})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: "Invalid input", Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.Reply(claims.ID, id, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Balasan tersimpan", Body: resp})
}
