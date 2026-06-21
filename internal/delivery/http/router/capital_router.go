package router

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/gofiber/fiber/v2"
)

func UseCapitalRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewCapitalInjectionController(ctx)

	// Admin only — setoran modal adalah keputusan finansial, kasir tidak boleh.
	cap := r.Group("/capital-injections", auth.AllowAdmins())
	cap.Get("/", ctrl.List)
	cap.Post("/", ctrl.Create)
	cap.Put("/:id", ctrl.Update)
	cap.Delete("/:id", ctrl.Delete)
}
