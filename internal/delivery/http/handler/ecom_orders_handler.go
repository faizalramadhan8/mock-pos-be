package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomOrdersController struct {
	Log     *zerolog.Logger
	Service *usecase.EcomOrdersService
}

func NewEcomOrdersController(ctx context.Context) *EcomOrdersController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomOrdersController{
		Log:     logger,
		Service: usecase.NewEcomOrdersService(ctx, db),
	}
}

func (ctrl *EcomOrdersController) List(c *fiber.Ctx) error {
	claims := c.Locals("session").(*dto.JWTClaims)
	rows, fail := ctrl.Service.List(claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: rows})
}

func (ctrl *EcomOrdersController) GetDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.GetDetail(claims.ID, id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// ConfirmReceived — customer klik "Barang Diterima". Body kosong — cukup
// order id di URL + JWT untuk ownership check di service.
func (ctrl *EcomOrdersController) ConfirmReceived(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.ConfirmReceived(claims.ID, id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Terima kasih! Pesanan ditandai selesai.", Body: resp})
}
