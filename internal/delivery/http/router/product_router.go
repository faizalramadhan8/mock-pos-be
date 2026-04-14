package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
)

func UseProductRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewProductController(ctx)

	products := r.Group("/products", auth.AllowAll())
	products.Get("/", ctrl.GetAll)
	products.Get("/low-stock", ctrl.GetLowStock)
	products.Get("/sku/:sku", ctrl.GetBySKU)
	products.Get("/:id", ctrl.GetByID)
	products.Post("/", auth.AllowInventoryWrite(), ctrl.Create)
	products.Put("/:id", auth.AllowInventoryWrite(), ctrl.Update)
	products.Patch("/:id/stock", auth.AllowInventoryWrite(), ctrl.AdjustStock)
	products.Patch("/:id/toggle-active", auth.AllowAdmins(), ctrl.ToggleActive)
	products.Delete("/:id", auth.AllowAdmins(), ctrl.Delete)
}

func UseCategoryRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewCategoryController(ctx)

	categories := r.Group("/categories", auth.AllowAll())
	categories.Get("/", ctrl.GetAll)
	categories.Post("/", auth.AllowAdmins(), ctrl.Create)
	categories.Put("/:id", auth.AllowAdmins(), ctrl.Update)
	categories.Delete("/:id", auth.AllowAdmins(), ctrl.Delete)
}

func UseSupplierRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewSupplierController(ctx)

	suppliers := r.Group("/suppliers", auth.AllowAll())
	suppliers.Get("/", ctrl.GetAll)
	suppliers.Get("/:id", ctrl.GetByID)
	suppliers.Post("/", auth.AllowInventoryWrite(), ctrl.Create)
	suppliers.Put("/:id", auth.AllowInventoryWrite(), ctrl.Update)
	suppliers.Delete("/:id", auth.AllowAdmins(), ctrl.Delete)
}
