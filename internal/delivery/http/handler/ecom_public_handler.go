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

// EcomPublicController — public endpoints untuk storefront. NO AUTH needed
// (customer browse produk sebelum login). Response filter enforce hanya
// produk yang tayang di ecom (ecom_is_available + stock_ecom > 0).
type EcomPublicController struct {
	Log     *zerolog.Logger
	Service *usecase.EcomPublicService
}

func NewEcomPublicController(ctx context.Context) *EcomPublicController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomPublicController{
		Log:     logger,
		Service: usecase.NewEcomPublicService(ctx, db),
	}
}

func (ctrl *EcomPublicController) ListCategories(c *fiber.Ctx) error {
	items, fail := ctrl.Service.ListCategories()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: items})
}

func (ctrl *EcomPublicController) ListProducts(c *fiber.Ctx) error {
	category := c.Query("category", "")
	search := c.Query("search", "")
	sort := c.Query("sort", "")
	cursor := c.Query("cursor", "")
	limit := c.QueryInt("limit", 24)
	if limit <= 0 || limit > 100 {
		limit = 24
	}

	resp, fail := ctrl.Service.ListProducts(category, search, sort, cursor, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *EcomPublicController) GetProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, fail := ctrl.Service.GetProduct(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}
