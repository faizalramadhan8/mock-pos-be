package handler

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	// Midtrans deprecated 28 Jul 2026 — kode di-preserve untuk rollback path.
	// "github.com/faizalramadhan/pos-be/internal/infrastructure/midtrans"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/pg"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomCheckoutController struct {
	Log         *zerolog.Logger
	Shipping    *usecase.EcomShippingService
	Checkout    *usecase.EcomCheckoutService
	AdminOrders *usecase.EcomAdminOrdersService // untuk webhook Biteship (butuh Biteship client + status mapping)
	Voucher     *usecase.EcomVoucherService
}

func NewEcomCheckoutController(ctx context.Context) *EcomCheckoutController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomCheckoutController{
		Log:         logger,
		Shipping:    usecase.NewEcomShippingService(ctx, db),
		Checkout:    usecase.NewEcomCheckoutService(ctx, db),
		AdminOrders: usecase.NewEcomAdminOrdersService(ctx, db),
		Voucher:     usecase.NewEcomVoucherService(ctx, db),
	}
}

// ValidateVoucher — customer input di checkout, kita hitung subtotal cart
// server-side (jangan trust FE subtotal), lalu validate voucher terhadap subtotal.
func (ctrl *EcomCheckoutController) ValidateVoucher(c *fiber.Ctx) error {
	var req dto.VoucherValidateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Invalid input", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	// Fetch cart subtotal via Shipping.getRates? Simpler: read cart directly.
	subtotal, ferr := ctrl.Checkout.CartSubtotal(claims.ID, nil)
	if ferr != nil {
		return c.Status(ferr.StatusCode.Code).JSON(dto.ApiResponse{
			Code: ferr.StatusCode.Code, Message: ferr.StatusCode.Message, Error: ferr.Message,
		})
	}
	resp, fail := ctrl.Voucher.Validate(req.Code, subtotal)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

func (ctrl *EcomCheckoutController) ShippingRates(c *fiber.Ctx) error {
	var req dto.ShippingRateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Shipping.GetRates(claims.ID, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

func (ctrl *EcomCheckoutController) CreateOrder(c *fiber.Ctx) error {
	var req dto.CheckoutCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error()})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Checkout.CreateOrder(claims.ID, claims.Email, claims.Phone, claims.Fullname, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Order created", Body: resp})
}

// PGWebhook — public endpoint, tanpa JWT. DOKU (via alifworks PG wrapper)
// POST notification tiap payment status change. Response format WAJIB DOKU-
// standard {responseCode, responseMessage} — kalau tidak, DOKU treat as
// failure dan retry infinitely.
//
// Signature verify (HMAC-SHA256 dari header `Signature`) di-skip untuk
// sekarang — sandbox tidak enforce, dan spec HMAC key belum di-share PG
// team. Prod TODO: verify pakai PGWebhookSecret.
//
// Kode Midtrans webhook lama di-preserve di git history — kalau perlu
// rollback tinggal git revert commit yang comment out midtrans import + call
// site di ecom_checkout_usecase.go.
func (ctrl *EcomCheckoutController) PGWebhook(c *fiber.Ctx) error {
	var notif pg.WebhookPayload
	if err := c.BodyParser(&notif); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"responseCode":    "5000700",
			"responseMessage": "Invalid payload",
		})
	}
	if fail := ctrl.Checkout.HandlePGNotification(notif); fail != nil {
		ctrl.Log.Warn().
			Str("invoice", notif.Order.InvoiceNumber).
			Str("status", notif.Transaction.Status).
			Str("err", fail.Message).
			Msg("PG webhook handle failed")
		// Tetap 200 supaya PG tidak retry infinite — log warn saja.
		return c.JSON(fiber.Map{
			"responseCode":    "2000700",
			"responseMessage": "Logged",
		})
	}
	ctrl.Log.Info().
		Str("invoice", notif.Order.InvoiceNumber).
		Str("status", notif.Transaction.Status).
		Str("channel", notif.Channel.ID).
		Msg("PG webhook processed OK")
	return c.JSON(fiber.Map{
		"responseCode":    "2000700",
		"responseMessage": "Success",
	})
}

// BiteshipWebhook — public endpoint. Biteship POST setiap status change
// order (allocated/picked/delivered/dst) + saat waybill_id di-assign kurir.
// Verify signature via HMAC-SHA256 dengan shared secret dari config webhook
// di Biteship dashboard. Bypass verify kalau BITESHIP_WEBHOOK_SECRET kosong
// (dev mode).
func (ctrl *EcomCheckoutController) BiteshipWebhook(c *fiber.Ctx) error {
	// Read raw body untuk signature verify — c.Body() = raw sebelum parse.
	body := c.Body()

	// Installation validation ping — Biteship (dan gateway lain) kirim POST
	// dengan body kosong / `{}` saat admin klik "Buat Webhook" untuk cek URL
	// reachable + return 200. HARUS respond OK tanpa verify signature, else
	// Biteship reject setup dengan error "URL doesn't respond with ok response".
	// Setelah setup jadi, real event bakal punya body non-empty + signature valid.
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || trimmed == "{}" {
		ctrl.Log.Info().Str("ip", c.IP()).Msg("Biteship webhook installation ping (empty body)")
		return c.JSON(fiber.Map{"status": "ok"})
	}

	// Biteship header signature — pakai header standar HMAC.
	// Docs: https://biteship.com/id/docs/api/webhook
	sig := c.Get("X-Biteship-Signature")
	if !ctrl.AdminOrders.Biteship.VerifyWebhookSignature(body, sig) {
		ctrl.Log.Warn().Str("ip", c.IP()).Msg("Biteship webhook signature mismatch — rejected")
		return c.Status(fiber.StatusUnauthorized).JSON(dto.ApiResponse{
			Code: fiber.StatusUnauthorized, Message: "Invalid signature",
		})
	}

	var payload usecase.BiteshipWebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Invalid payload", Error: err.Error(),
		})
	}
	payload.Normalize()

	if fail := ctrl.AdminOrders.HandleBiteshipWebhook(payload); fail != nil {
		ctrl.Log.Warn().
			Str("biteship_id", payload.OrderID).
			Str("ref_id", payload.ReferenceID).
			Str("err", fail.Message).
			Msg("Biteship webhook handle failed")
		// Return 200 supaya Biteship tidak retry infinitely — log warn saja.
		// (Kalau balik error, Biteship akan spam webhook queue.)
		return c.JSON(fiber.Map{"status": "logged"})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
