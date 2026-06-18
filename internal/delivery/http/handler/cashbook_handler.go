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

type CashbookController struct {
	Log     *zerolog.Logger
	Service *usecase.CashbookService
}

func NewCashbookController(ctx context.Context) *CashbookController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &CashbookController{
		Log:     logger,
		Service: usecase.NewCashbookService(ctx, db),
	}
}

// GetOpeningBalance GET /cashbook/opening?year=YYYY&month=MM
func (ctrl *CashbookController) GetOpeningBalance(c *fiber.Ctx) error {
	year, _ := strconv.Atoi(c.Query("year"))
	month, _ := strconv.Atoi(c.Query("month"))
	if year == 0 || month == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: "year & month required"})
	}
	resp, fail := ctrl.Service.GetOpeningBalance(year, month)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	// Body bisa null kalau belum di-set (FE treat sebagai 0)
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// SetOpeningBalance POST /cashbook/opening
func (ctrl *CashbookController) SetOpeningBalance(c *fiber.Ctx) error {
	var req dto.SetOpeningBalanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.SetOpeningBalance(req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.Status(fiber.StatusOK).JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// ListOpeningBalances GET /cashbook/opening/all
func (ctrl *CashbookController) ListOpeningBalances(c *fiber.Ctx) error {
	rows, fail := ctrl.Service.ListOpeningBalances()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: rows})
}
