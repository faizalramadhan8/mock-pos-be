package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/midtrans"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomCheckoutController struct {
	Log         *zerolog.Logger
	Shipping    *usecase.EcomShippingService
	Checkout    *usecase.EcomCheckoutService
}

func NewEcomCheckoutController(ctx context.Context) *EcomCheckoutController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomCheckoutController{
		Log:      logger,
		Shipping: usecase.NewEcomShippingService(ctx, db),
		Checkout: usecase.NewEcomCheckoutService(ctx, db),
	}
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

// MidtransWebhook — public endpoint, tanpa JWT. Midtrans POST notification tiap
// status change. Verify via signature_key (SHA512 hash) — untuk MVP skip signature
// karena Bu Santi belum register production Midtrans, cukup validate order exists.
func (ctrl *EcomCheckoutController) MidtransWebhook(c *fiber.Ctx) error {
	var notif midtrans.Notification
	if err := c.BodyParser(&notif); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{Code: fiber.ErrBadRequest.Code, Message: "Invalid payload"})
	}
	if fail := ctrl.Checkout.HandleMidtransNotification(notif); fail != nil {
		ctrl.Log.Warn().Str("order_id", notif.OrderID).Str("status", notif.TransactionStatus).Msg("Webhook handle failed")
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}
