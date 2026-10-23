package handler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminController handles admin panel untuk e-commerce Bu Santi.
// Login gated by role IN (ecom_admin, ecom_superadmin, superadmin).
// Product endpoints hanya bisa edit field ecom (ecom_price, ecom_is_available,
// dst). Stok TIDAK termasuk — sejak migration 000073 stok toko dan online satu
// angka, diubah lewat Sesuaikan Stok di POS supaya ada audit trail.
// POS field (stock, selling_price) tetap dikelola di POS Inventory —
// strict separation per keputusan Bu Santi 20 Jul 2026.
type EcomAdminController struct {
	Log              *zerolog.Logger
	AuthService      *usecase.AuthService
	EcomAdminSvc     *usecase.EcomAdminService
	EcomCategorySvc  *usecase.EcomCategoryService
	EcomOrdersSvc    *usecase.EcomAdminOrdersService
	EcomVoucherSvc   *usecase.EcomVoucherService
	EcomStatsSvc     *usecase.EcomAdminStatsService
	EcomCustomerSvc  *usecase.EcomAdminCustomersService
	EcomRefundSvc    *usecase.EcomAdminRefundsService
	EcomSettingsSvc  *usecase.EcomAdminSettingsService
	EcomBroadcastSvc *usecase.EcomAdminBroadcastsService
	EcomAnalyticsSvc *usecase.EcomAdminAnalyticsService
	EcomActivitySvc  *usecase.EcomAdminActivityService
	// Katalog produk toko rumah — CRUD penuh, terpisah dari katalog POS.
	EcomCatalogSvc *usecase.EcomAdminCatalogService
	// DB — untuk pembacaan ringan yang tidak butuh usecase sendiri
	// (mis. daftar kategori & pemasok untuk form produk).
	DB *gorm.DB
}

func NewEcomAdminController(ctx context.Context) *EcomAdminController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomAdminController{
		Log:              logger,
		AuthService:      usecase.NewAuthService(ctx, db),
		EcomAdminSvc:     usecase.NewEcomAdminService(ctx, db),
		EcomCategorySvc:  usecase.NewEcomCategoryService(ctx, db),
		EcomOrdersSvc:    usecase.NewEcomAdminOrdersService(ctx, db),
		EcomVoucherSvc:   usecase.NewEcomVoucherService(ctx, db),
		EcomStatsSvc:     usecase.NewEcomAdminStatsService(ctx, db),
		EcomCustomerSvc:  usecase.NewEcomAdminCustomersService(ctx, db),
		EcomRefundSvc:    usecase.NewEcomAdminRefundsService(ctx, db),
		EcomSettingsSvc:  usecase.NewEcomAdminSettingsService(ctx, db),
		EcomBroadcastSvc: usecase.NewEcomAdminBroadcastsService(ctx, db, usecase.NewPushService(ctx, db)),
		EcomAnalyticsSvc: usecase.NewEcomAdminAnalyticsService(ctx, db),
		EcomActivitySvc:  usecase.NewEcomAdminActivityService(ctx, db),
		EcomCatalogSvc:   usecase.NewEcomAdminCatalogService(ctx, db),
		DB:               db,
	}
}

// ============ Sprint 5 Chunk 10 — Bulk product ops (2 Aug 2026) ============

type bulkProductRequest struct {
	Action     string   `json:"action" validate:"required,oneof=publish unpublish sync_price reset_price"`
	ProductIDs []string `json:"product_ids" validate:"required,min=1"`
}

func (ctrl *EcomAdminController) BulkProductOps(c *fiber.Ctx) error {
	var req bulkProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.StatusBadRequest, Message: "Body tidak valid"})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err,
		})
	}
	var (
		resp *usecase.BulkUpdateResult
		fail *dto.ApiError
	)
	switch req.Action {
	case "publish":
		resp, fail = ctrl.EcomAdminSvc.BulkSetAvailable(req.ProductIDs, true)
	case "unpublish":
		resp, fail = ctrl.EcomAdminSvc.BulkSetAvailable(req.ProductIDs, false)
	case "sync_price":
		resp, fail = ctrl.EcomAdminSvc.BulkSyncPrice(req.ProductIDs)
	case "reset_price":
		resp, fail = ctrl.EcomAdminSvc.BulkResetPrice(req.ProductIDs)
	}
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	// Audit
	if claims, ok := c.Locals("session").(*dto.JWTClaims); ok {
		desc := fmt.Sprintf("Bulk %s %d produk", req.Action, resp.AffectedCount)
		ctrl.EcomActivitySvc.Record(claims.ID, "product_bulk_"+req.Action, "",
			desc, map[string]interface{}{"product_ids": req.ProductIDs})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// ============ Sprint 5 Chunk 9 — Activity log (2 Aug 2026) ============

func (ctrl *EcomAdminController) ListActivity(c *fiber.Ctx) error {
	action := c.Query("action", "")
	adminID := c.Query("admin_id", "")
	search := c.Query("search", "")
	limit := c.QueryInt("limit", 100)
	rows, fail := ctrl.EcomActivitySvc.List(action, adminID, search, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: rows})
}

// ============ Sprint 5 Chunk 8 — Analytics (2 Aug 2026) ============

func (ctrl *EcomAdminController) GetAnalytics(c *fiber.Ctx) error {
	resp, fail := ctrl.EcomAnalyticsSvc.Get()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// ============ Sprint 5 Chunk 6 — Broadcast push (2 Aug 2026) ============

func (ctrl *EcomAdminController) SendBroadcast(c *fiber.Ctx) error {
	var req dto.EcomBroadcastCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.StatusBadRequest, Message: "Body tidak valid"})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomBroadcastSvc.Send(claims.ID, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	ctrl.EcomActivitySvc.Record(claims.ID, "broadcast_sent", "broadcast:"+resp.ID,
		"Kirim broadcast push: "+req.Title,
		map[string]interface{}{"delivered": resp.DeliveredCount, "total": resp.TotalSubscribers})
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomAdminController) ListBroadcasts(c *fiber.Ctx) error {
	resp, fail := ctrl.EcomBroadcastSvc.List()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
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
	// Audit — cegah dispute "siapa ubah min order jadi Rp X kemarin".
	if claims, ok := c.Locals("session").(*dto.JWTClaims); ok {
		keys := make([]string, 0, len(patch))
		for k := range patch {
			keys = append(keys, k)
		}
		ctrl.EcomActivitySvc.Record(claims.ID, "settings_changed", "settings",
			"Ubah pengaturan storefront ("+joinKeys(keys)+")", patch)
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// joinKeys — helper untuk audit description. Limit 5 keys biar description
// tidak jadi paragraph.
func joinKeys(keys []string) string {
	if len(keys) == 0 {
		return "no changes"
	}
	if len(keys) > 5 {
		keys = append(keys[:5], "…")
	}
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
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
	ctrl.EcomActivitySvc.Record(claims.ID, "refund_created", "order:"+req.OrderID,
		fmt.Sprintf("Refund Rp %.0f (%s)", req.Amount, req.Method),
		map[string]interface{}{"amount": req.Amount, "method": req.Method, "complaint_id": req.ComplaintID})
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
	if claims, ok := c.Locals("session").(*dto.JWTClaims); ok {
		action := "customer_blocked"
		desc := "Blokir customer"
		if req.IsActive {
			action = "customer_unblocked"
			desc = "Aktifkan customer"
		}
		ctrl.EcomActivitySvc.Record(claims.ID, action, "customer:"+id, desc, nil)
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
// stoknya habis). Beda dari public storefront yang filter
// WHERE ecom_is_available=1 AND stock>0.
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

// UpdateEcomFields — patch HANYA field ecom_*. Field POS (termasuk stok)
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

// ─── Katalog produk toko rumah ────────────────────────────────────────────
//
// Panel rumah punya katalognya sendiri: produk yang dibuat di sini tidak
// pernah terlihat di POS, dan produk Bekasi tidak pernah terlihat di sini.
// Semua penjaga kepemilikannya ada di usecase — handler cuma meneruskan.

func (ctrl *EcomAdminController) CreateProduct(c *fiber.Ctx) error {
	var req dto.CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	// Kategori POS diisi backend, bukan ditanyakan ke admin — lihat
	// KategoriRumahID. Harus SEBELUM validasi, karena `category_id` wajib
	// di DTO yang dipakai bersama POS.
	if req.CategoryID == "" {
		id, fail := ctrl.EcomCatalogSvc.KategoriRumahID()
		if fail != nil {
			return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
				Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
			})
		}
		req.CategoryID = id
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Validation failed", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomCatalogSvc.Create(req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "successfully", Body: resp})
}

func (ctrl *EcomAdminController) UpdateProduct(c *fiber.Ctx) error {
	var req dto.UpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomCatalogSvc.Update(c.Params("id"), req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *EcomAdminController) DeleteProduct(c *fiber.Ctx) error {
	if fail := ctrl.EcomCatalogSvc.Delete(c.Params("id")); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully"})
}

// AdjustProductStock — satu-satunya jalur ubah stok rumah. Wajib alasan,
// set absolut, menulis jejak ke `stock_movements`. Sengaja TIDAK disatukan
// dengan UpdateProduct: form yang bisa menimpa stok diam-diam adalah
// persis mekanisme yang bikin insiden Sunbay 26 Jul 2026.
func (ctrl *EcomAdminController) AdjustProductStock(c *fiber.Ctx) error {
	var req dto.StockAdjustmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Validation failed", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomCatalogSvc.AdjustStock(c.Params("id"), req, claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// ListStockMovements — riwayat pergerakan stok toko rumah.
//
// Bu Santi melihat pergerakan stok rumah HANYA di sini (keputusan Fai
// 21 Okt 2026); POS menyaringnya keluar. `location` dipaksa "home" dan
// tidak dibaca dari query — panel rumah tidak boleh bisa mengintip
// pergerakan stok Bekasi dengan mengganti parameter.
func (ctrl *EcomAdminController) ListStockMovements(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "30"))
	movements, total, fail := ctrl.EcomCatalogSvc.Inventory.GetAllMovements(
		c.Query("type", ""), "home", page, limit,
	)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{
		Code: fiber.StatusOK, Message: "successfully", Body: movements,
		Meta: map[string]interface{}{"total": total, "page": page, "limit": limit},
	})
}

// ListCatalogRefs — kategori + pemasok POS untuk form produk panel rumah.
//
// Kategori dan pemasok dipakai BERSAMA oleh kedua tempat (keputusan Fai
// 21 Okt 2026): distributornya sama, dan dua daftar kategori terpisah
// berarti dua daftar yang harus dirawat. Yang terpisah cuma produknya.
//
// Dikirim dalam satu panggilan supaya form tidak perlu dua kali jalan.
func (ctrl *EcomAdminController) ListCatalogRefs(c *fiber.Ctx) error {
	type ref struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := struct {
		Categories []ref `json:"categories"`
		Suppliers  []ref `json:"suppliers"`
	}{Categories: []ref{}, Suppliers: []ref{}}

	// Galat TIDAK ditelan. Versi pertama memakai `if err == nil` lalu
	// mengembalikan daftar kosong dengan status 200 — layar menampilkan
	// dropdown kosong tanpa petunjuk apa pun, dan penyebabnya tidak bisa
	// dilacak dari mana-mana. Kategori kosong berarti produk tidak bisa
	// dibuat sama sekali, jadi ini kegagalan yang harus kedengaran.
	var cats []entity.Category
	if err := ctrl.DB.Order("name ASC").Find(&cats).Error; err != nil {
		ctrl.Log.Error().Err(err).Msg("gagal ambil kategori untuk form produk rumah")
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ApiResponse{
			Code: fiber.StatusInternalServerError, Message: "Gagal mengambil daftar kategori", Error: err.Error(),
		})
	}
	for _, c := range cats {
		out.Categories = append(out.Categories, ref{ID: c.ID, Name: c.Name})
	}

	var sups []entity.Supplier
	if err := ctrl.DB.Order("name ASC").Find(&sups).Error; err != nil {
		// Pemasok opsional — kegagalannya tidak boleh menghalangi pembuatan
		// produk, cukup dicatat. Beda dari kategori yang wajib diisi.
		ctrl.Log.Warn().Err(err).Msg("gagal ambil pemasok untuk form produk rumah")
	}
	for _, sp := range sups {
		out.Suppliers = append(out.Suppliers, ref{ID: sp.ID, Name: sp.Name})
	}

	ctrl.Log.Info().Int("kategori", len(out.Categories)).Int("pemasok", len(out.Suppliers)).
		Msg("catalog-refs untuk form produk rumah")
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: out})
}


// ─── Harga grosir produk rumah ────────────────────────────────────────────
//
// Semua penjaga kepemilikan ada di usecase: endpoint ini menolak id produk
// maupun id tier milik Bekasi dengan "tidak ditemukan", sehingga panel rumah
// tidak bisa menyenggol harga di konter kasir.

func (ctrl *EcomAdminController) ListProductTiers(c *fiber.Ctx) error {
	resp, fail := ctrl.EcomCatalogSvc.ListTiers(c.Params("id"))
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *EcomAdminController) CreateProductTier(c *fiber.Ctx) error {
	var req dto.SavePriceTierRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Validation failed", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomCatalogSvc.CreateTier(c.Params("id"), req, &claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{Code: fiber.StatusCreated, Message: "successfully", Body: resp})
}

func (ctrl *EcomAdminController) UpdateProductTier(c *fiber.Ctx) error {
	var req dto.SavePriceTierRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Validation failed", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.EcomCatalogSvc.UpdateTier(c.Params("tierId"), req, &claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *EcomAdminController) DeleteProductTier(c *fiber.Ctx) error {
	claims := c.Locals("session").(*dto.JWTClaims)
	if fail := ctrl.EcomCatalogSvc.DeleteTier(c.Params("tierId"), &claims.ID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully"})
}
