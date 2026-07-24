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

// EcomAdminController handles admin panel untuk e-commerce Bu Santi.
// Login gated by role IN (ecom_admin, ecom_superadmin, superadmin).
// Product endpoints hanya bisa edit field ecom (stock_ecom, ecom_price, dst).
// POS field (stock, selling_price) tetap dikelola di POS Inventory —
// strict separation per keputusan Bu Santi 20 Jul 2026.
type EcomAdminController struct {
	Log             *zerolog.Logger
	AuthService     *usecase.AuthService
	EcomAdminSvc    *usecase.EcomAdminService
	EcomCategorySvc *usecase.EcomCategoryService
}

func NewEcomAdminController(ctx context.Context) *EcomAdminController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomAdminController{
		Log:             logger,
		AuthService:     usecase.NewAuthService(ctx, db),
		EcomAdminSvc:    usecase.NewEcomAdminService(ctx, db),
		EcomCategorySvc: usecase.NewEcomCategoryService(ctx, db),
	}
}

// Login — reuse AuthService.Login untuk verify credential, tambah role gate.
// Cegah kasir/staff toko masuk ke admin panel ecom pakai credential mereka.
func (ctrl *EcomAdminController) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err,
		})
	}

	// Device fingerprint tidak relevan untuk ecom admin (ecom_admin/superadmin
	// tidak di-gate device binding). Kirim empty string ke Login.
	resp, _, fail := ctrl.AuthService.Login(req, c.Get(fiber.HeaderUserAgent), "")
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}

	// Role gate — kasir/staff toko tidak boleh masuk admin ecom.
	role := enum.Role(resp.User.Role)
	if role != enum.RoleEcomAdmin && role != enum.RoleEcomSuperAdmin && role != enum.RoleSuperAdmin {
		return c.Status(fiber.StatusForbidden).JSON(dto.ApiResponse{
			Code:    fiber.StatusForbidden,
			Message: "Access denied",
			Error:   "Akun ini bukan admin e-commerce",
		})
	}

	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// ListProducts — semua produk untuk admin manage (termasuk yang belum
// ke-allocate stock_ecom). Beda dari public storefront yang filter
// WHERE ecom_is_available=1 AND stock_ecom>0.
func (ctrl *EcomAdminController) ListProducts(c *fiber.Ctx) error {
	search := c.Query("search", "")
	cursor := c.Query("cursor", "")
	limit := c.QueryInt("limit", 100)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	items, nextCursor, fail := ctrl.EcomAdminSvc.ListProducts(search, cursor, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    map[string]interface{}{"items": items, "next_cursor": nextCursor},
	})
}

// GetProduct — single product detail untuk admin edit page.
func (ctrl *EcomAdminController) GetProduct(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, fail := ctrl.EcomAdminSvc.GetProduct(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// UpdateEcomFields — patch HANYA field ecom_* + stock_ecom. Field POS
// (stock, selling_price, member_price, name, sku, category, dst) TIDAK
// disentuh — dikelola di POS. Cegah admin ecom accidentally ubah harga POS.
func (ctrl *EcomAdminController) UpdateEcomFields(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.EcomFieldsUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err,
		})
	}

	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomAdminSvc.UpdateEcomFields(id, req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Product ecom fields updated", Body: resp})
}

// ─── Ecom Categories CRUD ────────────────────────────────────────────

func (ctrl *EcomAdminController) ListCategories(c *fiber.Ctx) error {
	items, fail := ctrl.EcomCategorySvc.List(false)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: items})
}

func (ctrl *EcomAdminController) CreateCategory(c *fiber.Ctx) error {
	var req dto.EcomCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	resp, fail := ctrl.EcomCategorySvc.Create(req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "Category created", Body: resp})
}

func (ctrl *EcomAdminController) UpdateCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.EcomCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	resp, fail := ctrl.EcomCategorySvc.Update(id, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Category updated", Body: resp})
}

func (ctrl *EcomAdminController) DeleteCategory(c *fiber.Ctx) error {
	id := c.Params("id")
	if fail := ctrl.EcomCategorySvc.Delete(id); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Category deleted"})
}
