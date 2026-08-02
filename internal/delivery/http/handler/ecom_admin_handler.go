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
	EcomOrdersSvc   *usecase.EcomAdminOrdersService
	EcomVoucherSvc  *usecase.EcomVoucherService
	EcomStatsSvc    *usecase.EcomAdminStatsService
	EcomCustomerSvc *usecase.EcomAdminCustomersService
	EcomRefundSvc   *usecase.EcomAdminRefundsService
	EcomSettingsSvc *usecase.EcomAdminSettingsService
}

func NewEcomAdminController(ctx context.Context) *EcomAdminController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomAdminController{
		Log:             logger,
		AuthService:     usecase.NewAuthService(ctx, db),
		EcomAdminSvc:    usecase.NewEcomAdminService(ctx, db),
		EcomCategorySvc: usecase.NewEcomCategoryService(ctx, db),
		EcomOrdersSvc:   usecase.NewEcomAdminOrdersService(ctx, db),
		EcomVoucherSvc:  usecase.NewEcomVoucherService(ctx, db),
		EcomStatsSvc:    usecase.NewEcomAdminStatsService(ctx, db),
		EcomCustomerSvc: usecase.NewEcomAdminCustomersService(ctx, db),
		EcomRefundSvc:   usecase.NewEcomAdminRefundsService(ctx, db),
		EcomSettingsSvc: usecase.NewEcomAdminSettingsService(ctx, db),
	}
}

// ============ Sprint 4 Chunk 5 — Ecom Settings (31 Jul 2026) ============

func (ctrl *EcomAdminController) GetSettings(c *fiber.Ctx) error {
	resp, fail := ctrl.EcomSettingsSvc.Get()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) UpdateSettings(c *fiber.Ctx) error {
	patch := map[string]interface{}{}
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.StatusBadRequest, Message: "Body tidak valid"})
	}
	resp, fail := ctrl.EcomSettingsSvc.Update(patch)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// ============ Sprint 4 Chunk 2 — Refund flow (31 Jul 2026) ============

func (ctrl *EcomAdminController) CreateRefund(c *fiber.Ctx) error {
	var req dto.EcomRefundCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.StatusBadRequest, Message: "Body tidak valid"})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomRefundSvc.Create(claims.ID, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) ListRefundsByOrder(c *fiber.Ctx) error {
	orderID := c.Params("id")
	resp, fail := ctrl.EcomRefundSvc.ListByOrder(orderID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// ============ Sprint 4 Chunk 1 — Customer management (30 Jul 2026) ============

func (ctrl *EcomAdminController) ListCustomers(c *fiber.Ctx) error {
	search := c.Query("search", "")
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 25)
	resp, fail := ctrl.EcomCustomerSvc.List(search, page, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) GetCustomer(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, fail := ctrl.EcomCustomerSvc.GetDetail(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) SetCustomerActive(c *fiber.Ctx) error {
	id := c.Params("id")
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.StatusBadRequest, Message: "Body tidak valid"})
	}
	if fail := ctrl.EcomCustomerSvc.SetActive(id, req.IsActive); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK"})
}

// GetDashboardStats — Sprint 3 #13.
func (ctrl *EcomAdminController) GetDashboardStats(c *fiber.Ctx) error {
	resp, fail := ctrl.EcomStatsSvc.GetDashboard()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) GetLowStock(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 10)
	resp, fail := ctrl.EcomStatsSvc.GetLowStock(limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
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

// ─── Ecom Admin Orders (Sprint 1) ────────────────────────────────────

func (ctrl *EcomAdminController) ListOrders(c *fiber.Ctx) error {
	status := c.Query("status", "")
	search := c.Query("search", "")
	cursor := c.Query("cursor", "")
	limit := c.QueryInt("limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	resp, fail := ctrl.EcomOrdersSvc.List(status, search, cursor, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) GetOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	resp, fail := ctrl.EcomOrdersSvc.GetDetail(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) UpdateOrderStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.AdminUpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Invalid status", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomOrdersSvc.UpdateStatus(id, req.Status, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Status updated", Body: resp})
}

// CreateBiteshipShipment — admin klik "Buat Order Biteship" di detail page.
// Trigger Biteship Order API → save order_id + status → tunggu webhook untuk
// waybill (kadang langsung ada di response).
func (ctrl *EcomAdminController) CreateBiteshipShipment(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomOrdersSvc.CreateBiteshipShipment(id, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Shipment Biteship dibuat", Body: resp})
}

// SyncBiteshipStatus — admin klik "Sync dari Biteship" saat webhook missed /
// status stuck. Panggil GET /v1/orders/:id untuk fresh state.
func (ctrl *EcomAdminController) SyncBiteshipStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomOrdersSvc.SyncBiteshipStatus(id, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Status Biteship di-sync", Body: resp})
}

// GetBiteshipBalance — admin dashboard widget saldo Biteship. Alert kalau
// balance < 100k supaya Bu Santi top-up sebelum order tetap gagal.
func (ctrl *EcomAdminController) GetBiteshipBalance(c *fiber.Ctx) error {
	balance, err := ctrl.EcomOrdersSvc.Biteship.GetBalance()
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(dto.ApiResponse{
			Code: fiber.StatusBadGateway, Message: "Gagal ambil saldo", Error: err.Error(),
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: balance})
}

func (ctrl *EcomAdminController) SetOrderShipping(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.AdminSetShippingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Invalid resi", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomOrdersSvc.SetShipping(id, req.AWB, req.Courier, req.Service, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Resi tersimpan, status shipped", Body: resp})
}

// ─── Vouchers CRUD (Sprint 5) ─────────────────────────────────────────

func (ctrl *EcomAdminController) ListVouchers(c *fiber.Ctx) error {
	items, fail := ctrl.EcomVoucherSvc.List()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: items})
}

func (ctrl *EcomAdminController) CreateVoucher(c *fiber.Ctx) error {
	var req dto.VoucherCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: "Invalid input", Error: err})
	}
	v, fail := ctrl.EcomVoucherSvc.Create(req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "Voucher created", Body: v})
}

func (ctrl *EcomAdminController) UpdateVoucher(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.VoucherCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error()})
	}
	v, fail := ctrl.EcomVoucherSvc.Update(id, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Voucher updated", Body: v})
}

func (ctrl *EcomAdminController) DeleteVoucher(c *fiber.Ctx) error {
	id := c.Params("id")
	if fail := ctrl.EcomVoucherSvc.Delete(id); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Voucher deleted"})
}
