package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

func UseRouter(ctx context.Context, r fiber.Router) {

	prefix := r.Group("/api/v1")

	// Auth & Users
	UseAuthRouter(ctx, prefix)

	// Products, Categories, Suppliers
	UseProductRouter(ctx, prefix)
	UseCategoryRouter(ctx, prefix)
	UseSupplierRouter(ctx, prefix)

	// Orders & Refunds
	UseOrderRouter(ctx, prefix)
	UseRefundRouter(ctx, prefix)

	// Inventory (Stock Movements & Batches)
	UseInventoryRouter(ctx, prefix)

	// Members
	UseMemberRouter(ctx, prefix)

	// Cash Sessions
	UseCashSessionRouter(ctx, prefix)

	// Audit Log
	UseAuditRouter(ctx, prefix)

	// Settings & Bank Accounts
	UseSettingsRouter(ctx, prefix)

	// Dashboard
	UseDashboardRouter(ctx, prefix)

	// Upload
	UseUploadRouter(ctx, prefix)

	// Push Notifications
	UsePushRouter(ctx, prefix)
}
