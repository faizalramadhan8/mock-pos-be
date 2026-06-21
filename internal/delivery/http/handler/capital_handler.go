package handler

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CapitalInjectionController struct {
	Service *usecase.CapitalInjectionService
}

func NewCapitalInjectionController(ctx context.Context) *CapitalInjectionController {
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &CapitalInjectionController{Service: usecase.NewCapitalInjectionService(ctx, db)}
}

func (ctrl *CapitalInjectionController) List(c *fiber.Ctx) error {
	from := c.Query("from")
	to := c.Query("to")
	rows, fail := ctrl.Service.List(from, to)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: rows})
}

func (ctrl *CapitalInjectionController) Create(c *fiber.Ctx) error {
	var req dto.SaveCapitalInjectionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	resp, fail := ctrl.Service.Create(req, userIDFromCtx(c))
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "successfully", Body: resp})
}

func (ctrl *CapitalInjectionController) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.SaveCapitalInjectionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	resp, fail := ctrl.Service.Update(id, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *CapitalInjectionController) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if fail := ctrl.Service.Delete(id); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Capital injection deleted"})
}
