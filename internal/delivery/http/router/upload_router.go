package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
)

func UseUploadRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewUploadController(ctx)

	upload := r.Group("/upload", auth.AllowAll())
	upload.Post("/", ctrl.UploadImage)
}
