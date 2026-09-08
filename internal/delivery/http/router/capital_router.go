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

	// SUPERADMIN ONLY (8 Sep 2026) — di-tighten dari AllowAdmins per request
	// Bu Santi: "Arus kas dan laba rugi, selain saya tidak ada yang bisa lihat.
	// Termasuk Pak Komar dan siapapun yang punya akses admin. Aksesnya terbatas
	// karna bersifat rahasia." Modal/prive owner = data keuangan pribadi.
	// FE gate (header icon hidden) tidak cukup — admin bisa hit API langsung
	// via DevTools, jadi BE harus tolak juga.
	cap := r.Group("/capital-injections", auth.AllowSuperAdmin())
	cap.Get("/", ctrl.List)
	cap.Post("/", ctrl.Create)
	cap.Put("/:id", ctrl.Update)
	cap.Delete("/:id", ctrl.Delete)
}
