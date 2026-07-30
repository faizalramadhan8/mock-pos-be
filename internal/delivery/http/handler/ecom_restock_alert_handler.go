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

type EcomRestockAlertController struct {
	Log     *zerolog.Logger
	Service *usecase.EcomRestockAlertService
}

func NewEcomRestockAlertController(ctx context.Context) *EcomRestockAlertController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomRestockAlertController{
		Log:     logger,
		Service: usecase.NewEcomRestockAlertService(ctx, db),
	}
}

// Subscribe — POST /ecom/products/:id/restock-alert
func (ctrl *EcomRestockAlertController) Subscribe(c *fiber.Ctx) error {
	productID := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	if fail := ctrl.Service.Subscribe(claims.ID, productID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Kami akan kabari saat produk restock"})
}

// Unsubscribe — DELETE /ecom/products/:id/restock-alert
func (ctrl *EcomRestockAlertController) Unsubscribe(c *fiber.Ctx) error {
	productID := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	if fail := ctrl.Service.Unsubscribe(claims.ID, productID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Alert dimatikan"})
}

// Status — GET /ecom/products/:id/restock-alert (cek subscribe status)
func (ctrl *EcomRestockAlertController) Status(c *fiber.Ctx) error {
	productID := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	subscribed := ctrl.Service.IsSubscribed(claims.ID, productID)
	return c.JSON(dto.ApiResponse{
		Code: fiber.StatusOK, Message: "OK",
		Body: fiber.Map{"subscribed": subscribed},
	})
}

// DispatchNotif — POST /ecom/admin/products/:id/dispatch-restock. Admin
// trigger manual saat konfirmasi produk restock. Cron nightly juga bisa
// scan produk yang restock + trigger otomatis, tapi manual endpoint jadi
// safety net + testing tool.
func (ctrl *EcomRestockAlertController) DispatchNotif(c *fiber.Ctx) error {
	productID := c.Params("id")
	go ctrl.Service.TriggerRestockNotif(productID) // async, best-effort
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Notif restock sedang diproses"})
}
