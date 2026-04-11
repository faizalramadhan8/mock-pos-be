package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
)

func UseOrderRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewOrderController(ctx)

	orders := r.Group("/orders", auth.AllowAll())
	orders.Get("/", ctrl.GetAll)
	orders.Get("/stats", ctrl.GetStats)
	orders.Get("/:id", ctrl.GetByID)
	orders.Post("/", auth.AllowCashier(), ctrl.Create)
	orders.Patch("/:id/cancel", auth.AllowAdmins(), ctrl.Cancel)
}

func UseInventoryRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewInventoryController(ctx)

	inventory := r.Group("/inventory", auth.AllowAll())
	inventory.Get("/movements", ctrl.GetAllMovements)
	inventory.Post("/movements", auth.AllowInventoryWrite(), ctrl.CreateMovement)
	inventory.Patch("/movements/:id/payment-status", auth.AllowInventoryWrite(), ctrl.UpdatePaymentStatus)
	inventory.Get("/batches", ctrl.GetAllBatches)
	inventory.Get("/batches/expiring", ctrl.GetExpiringBatches)
	inventory.Post("/batches/consume-fifo", auth.AllowInventoryWrite(), ctrl.ConsumeFIFO)
}

func UseRefundRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewRefundController(ctx)

	refunds := r.Group("/refunds", auth.AllowAdmins())
	refunds.Post("/", ctrl.Create)
	refunds.Get("/order/:orderId", ctrl.GetByOrderID)
}
