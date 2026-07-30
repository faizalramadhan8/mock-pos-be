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
	// Ecom categories CRUD (terpisah dari POS categories).
	gated.Get("/categories", ctrl.ListCategories)
	gated.Post("/categories", ctrl.CreateCategory)
	gated.Put("/categories/:id", ctrl.UpdateCategory)
	gated.Delete("/categories/:id", ctrl.DeleteCategory)
	// Ecom orders (Sprint 1) — admin fulfillment panel.
	gated.Get("/orders", ctrl.ListOrders)
	gated.Get("/orders/:id", ctrl.GetOrder)
	gated.Patch("/orders/:id/status", ctrl.UpdateOrderStatus)
	gated.Post("/orders/:id/shipping", ctrl.SetOrderShipping)
	// Sprint 3 — Biteship auto-resi.
	gated.Post("/orders/:id/biteship-create", ctrl.CreateBiteshipShipment)
	// 28 Jul 2026 — admin sync status manual (fallback webhook missed).
	gated.Post("/orders/:id/biteship-sync", ctrl.SyncBiteshipStatus)
	// Balance widget (dashboard).
	gated.Get("/biteship/balance", ctrl.GetBiteshipBalance)
	// Sprint 1 #4 — Komplain admin panel.
	complaintAdminCtrl := handler.NewEcomComplaintController(ctx)
	gated.Get("/complaints", complaintAdminCtrl.ListForAdmin)
	gated.Patch("/complaints/:id", complaintAdminCtrl.Reply)

	// Sprint 3 #14 — Review moderation admin.
	reviewAdminCtrl := handler.NewEcomReviewController(ctx)
	gated.Get("/reviews", reviewAdminCtrl.ListForAdmin)
	gated.Patch("/reviews/:id", reviewAdminCtrl.ToggleHide)

	// Sprint 3 #13 — Admin dashboard stats + low stock alert
	gated.Get("/dashboard", ctrl.GetDashboardStats)
	gated.Get("/low-stock", ctrl.GetLowStock)

	// Sprint 3 #16 — Admin trigger restock notif dispatcher
	restockAdminCtrl := handler.NewEcomRestockAlertController(ctx)
	gated.Post("/products/:id/dispatch-restock", restockAdminCtrl.DispatchNotif)
	// Sprint 5 — Voucher/Promo CRUD (admin only).
	gated.Get("/vouchers", ctrl.ListVouchers)
	gated.Post("/vouchers", ctrl.CreateVoucher)
	gated.Put("/vouchers/:id", ctrl.UpdateVoucher)
	gated.Delete("/vouchers/:id", ctrl.DeleteVoucher)
}

// UseEcomPublicRouter — public storefront endpoints (no auth). Customer
// browse produk sebelum login. Filter enforce ecom_is_available + stock_ecom>0.
func UseEcomPublicRouter(ctx context.Context, r fiber.Router) {
	ctrl := handler.NewEcomPublicController(ctx)
	reviewCtrl := handler.NewEcomReviewController(ctx)
	checkoutCtrl := handler.NewEcomCheckoutController(ctx)
	g := r.Group("/ecom")
	g.Get("/categories", ctrl.ListCategories)
	g.Get("/products", ctrl.ListProducts)
	g.Get("/products/:id", ctrl.GetProduct)
	// Sprint 2 #6 (30 Jul 2026) — Related products di PDP.
	g.Get("/products/:id/related", ctrl.GetRelated)
	// Sprint 5b — Reviews public listing.
	g.Get("/products/:id/reviews", reviewCtrl.ListForProduct)
	// Webhooks — mount di public group supaya BEBAS middleware AllowEcomCustomer.
	// Dulu di UseEcomCustomerRouter (via r.Post parent) tapi tetap kena JWT check
	// karena Fiber Group middleware apply per prefix. Bu Santi 28 Jul 2026.
	//   - Biteship: kurir kirim event tracking (verify HMAC di handler)
	//   - PG DOKU:  payment gateway kirim status change (DOKU-standard response)
	g.Post("/payments/webhook/pg", checkoutCtrl.PGWebhook)
	g.Post("/shipping/webhook/biteship", checkoutCtrl.BiteshipWebhook)
	// Maps API — public (dipakai FE address form saat register / add address).
	// Rate limit di Biteship side; kita cache 60s di FE untuk cegah spam.
	g.Get("/shipping/areas", checkoutCtrl.SearchAreas)
}

// UseEcomCustomerRouter — endpoints untuk authenticated customer (cart, address,
// order). Role 'user' + superadmin. Bu Santi 24 Jul 2026.
func UseEcomCustomerRouter(ctx context.Context, r fiber.Router) {
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	cartCtrl := handler.NewEcomCartController(ctx)
	addrCtrl := handler.NewEcomAddressController(ctx)
	checkoutCtrl := handler.NewEcomCheckoutController(ctx)
	ordersCtrl := handler.NewEcomOrdersController(ctx)
	reviewCtrl := handler.NewEcomReviewController(ctx)

	g := r.Group("/ecom", auth.AllowEcomCustomer())
	// Cart
	g.Get("/cart", cartCtrl.GetCart)
	g.Post("/cart/items", cartCtrl.AddItem)
	g.Patch("/cart/items/:id", cartCtrl.UpdateItem)
	g.Delete("/cart/items/:id", cartCtrl.RemoveItem)
	// Address book
	g.Get("/addresses", addrCtrl.List)
	g.Post("/addresses", addrCtrl.Create)
	g.Put("/addresses/:id", addrCtrl.Update)
	g.Delete("/addresses/:id", addrCtrl.Delete)
	// Checkout
	g.Post("/shipping/rates", checkoutCtrl.ShippingRates)
	g.Post("/checkout/create-order", checkoutCtrl.CreateOrder)
	// Sprint 5 — Voucher validate (customer at checkout).
	g.Post("/checkout/validate-voucher", checkoutCtrl.ValidateVoucher)
	// Customer own orders
	g.Get("/orders", ordersCtrl.List)
	g.Get("/orders/:id", ordersCtrl.GetDetail)
	// Marketplace-pattern buyer confirmation (28 Jul 2026) — kurir tag
	// delivered → customer harus konfirmasi manual → completed.
	g.Post("/orders/:id/confirm-received", ordersCtrl.ConfirmReceived)
	// Sprint 1 (30 Jul 2026): customer self-service cancel + retry payment.
	g.Post("/orders/:id/cancel", ordersCtrl.CancelPending)
	g.Post("/orders/:id/retry-payment", ordersCtrl.RetryPayment)

	// Sprint 5b — Reviews (customer submit + check eligibility).
	g.Get("/products/:id/reviews/me", reviewCtrl.CanReviewMe)
	g.Post("/reviews", reviewCtrl.Submit)

	// Sprint 1 #4 (30 Jul 2026) — Komplain customer.
	complaintCtrl := handler.NewEcomComplaintController(ctx)
	g.Post("/complaints", complaintCtrl.Submit)
	g.Get("/complaints", complaintCtrl.ListForUser)

	// Sprint 3 #16 (30 Jul 2026) — Restock alert subscribe/status/unsub
	restockCtrl := handler.NewEcomRestockAlertController(ctx)
	g.Get("/products/:id/restock-alert", restockCtrl.Status)
	g.Post("/products/:id/restock-alert", restockCtrl.Subscribe)
	g.Delete("/products/:id/restock-alert", restockCtrl.Unsubscribe)

	// Webhooks Biteship + PG di-mount di UseEcomPublicRouter supaya BEBAS
	// middleware AllowEcomCustomer. Register di sini dulu bikin 401 karena
	// Fiber Group middleware apply per prefix `/ecom/*`, meskipun route di
	// register di parent r. Bu Santi 28 Jul 2026.
}
