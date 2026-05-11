package router

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/gofiber/fiber/v2"
)

// UseExpenseRouter — endpoint pengeluaran operasional + profit-loss.
// Read access: AllowAll (cashier boleh lihat widget di Dashboard). Write
// access: AllowAdmins (cuma admin/superadmin yang boleh catat/edit/hapus
// pengeluaran — supaya audit trail keuangan tidak dirusak kasir).
func UseExpenseRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewExpenseController(ctx)

	exp := r.Group("/expenses", auth.AllowAll())
	exp.Get("/", ctrl.List)
	exp.Get("/profit-loss", ctrl.ProfitLoss)
	exp.Get("/categories", ctrl.ListCategories)
	exp.Post("/", auth.AllowAdmins(), ctrl.Create)
	exp.Put("/:id", auth.AllowAdmins(), ctrl.Update)
	exp.Delete("/:id", auth.AllowAdmins(), ctrl.Delete)
	exp.Post("/categories", auth.AllowAdmins(), ctrl.CreateCategory)
	exp.Put("/categories/:id", auth.AllowAdmins(), ctrl.UpdateCategory)
}
