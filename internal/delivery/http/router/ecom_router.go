package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
)

// UseEcomAdminRouter mount endpoints untuk admin panel ecom (Bu Santi 20 Jul
// 2026). Gate role via AllowEcomAdmins (ecom_admin/ecom_superadmin/superadmin).
// Public login endpoint tanpa auth — validasi role terjadi di handler setelah
// verify credential.
func UseEcomAdminRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewEcomAdminController(ctx)

	ecomAdmin := r.Group("/ecom/admin")

	// Public — login. Handler validate role setelah credential verified.
	ecomAdmin.Post("/auth/login", ctrl.Login)

	// Gated — semua endpoint di sini butuh ecom admin scope.
	gated := ecomAdmin.Group("/", auth.AllowEcomAdmins())
	gated.Get("/products", ctrl.ListProducts)
	gated.Get("/products/:id", ctrl.GetProduct)
	gated.Patch("/products/:id/ecom-fields", ctrl.UpdateEcomFields)
}
