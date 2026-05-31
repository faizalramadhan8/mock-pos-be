package router

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/gofiber/fiber/v2"
)

// UsePurchaseInvoiceRouter wires endpoints untuk Faktur Pembelian.
// Hanya admin/superadmin yang boleh create + delete (sensitif: bisa
// nge-update stok products). Read access untuk semua role yang AllowAll.
func UsePurchaseInvoiceRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewPurchaseInvoiceController(ctx)

	pi := r.Group("/purchase-invoices", auth.AllowAll())
	pi.Get("/", ctrl.GetAll)
	pi.Get("/:id", ctrl.GetByID)
	pi.Post("/", auth.AllowAdmins(), ctrl.Create)
	pi.Put("/:id", auth.AllowAdmins(), ctrl.Update)
	pi.Post("/:id/mark-paid", auth.AllowAdmins(), ctrl.MarkAsPaid)
	pi.Delete("/:id", auth.AllowAdmins(), ctrl.Delete)
}
