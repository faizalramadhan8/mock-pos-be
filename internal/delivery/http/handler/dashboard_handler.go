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

type DashboardController struct {
	Log     *zerolog.Logger
	Service *usecase.DashboardService
}

func NewDashboardController(ctx context.Context) *DashboardController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &DashboardController{
		Log:     logger,
		Service: usecase.NewDashboardService(ctx, db),
	}
}

func (ctrl *DashboardController) Get(c *fiber.Ctx) error {
	claims := c.Locals("session").(*dto.JWTClaims)

	dashboard, fail := ctrl.Service.GetDashboard(claims.Role)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: dashboard})
}
