package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type ProductController struct {
	Log     *zerolog.Logger
	Service *usecase.ProductService
}

func NewProductController(ctx context.Context) *ProductController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &ProductController{
		Log:     logger,
		Service: usecase.NewProductService(ctx, db),
	}
}

func (ctrl *ProductController) GetAll(c *fiber.Ctx) error {
	search := c.Query("search", "")
	categoryID := c.Query("category_id", "")
	supplierID := c.Query("supplier_id", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	products, total, fail := ctrl.Service.GetAll(search, categoryID, supplierID, page, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    products,
		Meta: map[string]interface{}{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (ctrl *ProductController) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	product, fail := ctrl.Service.GetByID(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: product})
}

func (ctrl *ProductController) GetBySKU(c *fiber.Ctx) error {
	sku := c.Params("sku")
	product, fail := ctrl.Service.GetBySKU(sku)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: product})
}

// NextSKU — return next auto-generated SKU untuk prefix tertentu. Endpoint
// ini aware soft-deleted SKUs (pakai Unscoped query di repo) supaya FE auto-
// gen tidak collide dengan SKU yang stuck di tombstone row.
func (ctrl *ProductController) NextSKU(c *fiber.Ctx) error {
	prefix := strings.ToUpper(strings.TrimSpace(c.Query("prefix", "")))
	sku, fail := ctrl.Service.NextSKU(prefix)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: map[string]string{"sku": sku}})
}

func (ctrl *ProductController) GetLowStock(c *fiber.Ctx) error {
	products, fail := ctrl.Service.GetLowStock()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: products})
}

func (ctrl *ProductController) Create(c *fiber.Ctx) error {
	var req dto.CreateProductRequest
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

func (ctrl *ProductController) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}

	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.Update(id, req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *ProductController) AdjustStock(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.AdjustStockRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}

	resp, fail := ctrl.Service.AdjustStock(id, req.Delta)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *ProductController) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if fail := ctrl.Service.Delete(id); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Product deleted successfully"})
}

func (ctrl *ProductController) GetPriceHistory(c *fiber.Ctx) error {
	id := c.Params("id")
	priceType := c.Query("price_type", "")
	rows, fail := ctrl.Service.GetPriceHistory(id, priceType)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: rows})
}

func (ctrl *ProductController) ToggleActive(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, fail := ctrl.Service.ToggleActive(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// ListPriceTiers GET /products/:id/tiers
func (ctrl *ProductController) ListPriceTiers(c *fiber.Ctx) error {
	id := c.Params("id")
	tiers, fail := ctrl.Service.ListPriceTiers(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: tiers})
}

// ListTierHistory GET /products/:id/tier-history — audit trail tier CRUD.
func (ctrl *ProductController) ListTierHistory(c *fiber.Ctx) error {
	id := c.Params("id")
	rows, fail := ctrl.Service.ListTierHistory(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: rows})
}

// userIDFromCtx — extract user_id dari JWT session claims di Fiber locals.
// Return nil kalau tidak ada (akan jadi NULL di changed_by).
func userIDFromCtx(c *fiber.Ctx) *string {
	v := c.Locals("session")
	if v == nil {
		return nil
	}
	claims, ok := v.(*dto.JWTClaims)
	if !ok || claims.ID == "" {
		return nil
	}
	id := claims.ID
	return &id
}

// CreatePriceTier POST /products/:id/tiers
func (ctrl *ProductController) CreatePriceTier(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.SavePriceTierRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	resp, fail := ctrl.Service.CreatePriceTier(id, req, userIDFromCtx(c))
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "successfully", Body: resp})
}

// UpdatePriceTier PUT /products/:id/tiers/:tierId
func (ctrl *ProductController) UpdatePriceTier(c *fiber.Ctx) error {
	tierID := c.Params("tierId")
	var req dto.SavePriceTierRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	resp, fail := ctrl.Service.UpdatePriceTier(tierID, req, userIDFromCtx(c))
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// DeletePriceTier DELETE /products/:id/tiers/:tierId
func (ctrl *ProductController) DeletePriceTier(c *fiber.Ctx) error {
	tierID := c.Params("tierId")
	if fail := ctrl.Service.DeletePriceTier(tierID, userIDFromCtx(c)); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Tier deleted"})
}

// SetRedeemable PATCH /products/:id/redeemable
// Body: {"is_redeemable": true|false}
func (ctrl *ProductController) SetRedeemable(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		IsRedeemable bool `json:"is_redeemable"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	resp, fail := ctrl.Service.SetRedeemable(id, req.IsRedeemable)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}
