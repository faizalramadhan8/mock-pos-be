package router

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/gofiber/fiber/v2"
)

func UseRedeemableRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewRedeemableItemController(ctx)

	items := r.Group("/redeemable-items", auth.AllowAll())
	// GET: kasir + admin bisa lihat (untuk POS browse + admin manage)
	items.Get("/", ctrl.GetAll)
	items.Get("/active", ctrl.GetActive)
	// CUD: admin only
	items.Post("/", auth.AllowAdmins(), ctrl.Create)
	items.Put("/:id", auth.AllowAdmins(), ctrl.Update)
	items.Delete("/:id", auth.AllowAdmins(), ctrl.Delete)
}
