package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
)

func UseMemberRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewMemberController(ctx)

	members := r.Group("/members", auth.AllowAll())
	members.Get("/", ctrl.GetAll)
	members.Get("/search", ctrl.SearchByPhone)
	members.Get("/:id/stats", auth.AllowAdmins(), ctrl.GetStats)
	members.Get("/:id/point-movements", ctrl.GetPointMovements)
	members.Post("/", ctrl.Create)
	members.Put("/:id", ctrl.Update)
	members.Delete("/:id", ctrl.Delete)
}

func UseCashSessionRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewCashSessionController(ctx)

	sessions := r.Group("/cash-sessions", auth.AllowCashier())
	sessions.Get("/", ctrl.GetAll)
	sessions.Get("/open", ctrl.GetOpen)
	sessions.Post("/", ctrl.Open)
	sessions.Patch("/:id/close", ctrl.Close)
}

func UseAuditRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewAuditController(ctx)

	audit := r.Group("/audit", auth.AllowAdmins())
	audit.Get("/", ctrl.GetAll)
	audit.Post("/", auth.AllowAll(), ctrl.Create)
}

func UseSettingsRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewSettingsController(ctx)

	settings := r.Group("/settings", auth.AllowAll())
	settings.Get("/", ctrl.Get)
	settings.Put("/", auth.AllowAdmins(), ctrl.Update)
	settings.Post("/bank-accounts", auth.AllowAdmins(), ctrl.AddBankAccount)
	settings.Delete("/bank-accounts/:id", auth.AllowAdmins(), ctrl.DeleteBankAccount)
}

func UseDashboardRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewDashboardController(ctx)

	dashboard := r.Group("/dashboard", auth.AllowAll())
	dashboard.Get("/", ctrl.Get)
}

// UseCashbookRouter — endpoint untuk Arus Kas opening balance per bulan.
// Admin-only (laporan keuangan sensitif). FE aggregate ledger client-side
// dari orders/expenses/invoices/refunds (sudah ada di stores) supaya tidak
// perlu endpoint aggregate baru di BE.
func UseCashbookRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewCashbookController(ctx)

	cashbook := r.Group("/cashbook", auth.AllowAdmins())
	cashbook.Get("/opening", ctrl.GetOpeningBalance)
	cashbook.Get("/opening/all", ctrl.ListOpeningBalances)
	cashbook.Post("/opening", ctrl.SetOpeningBalance)
}
