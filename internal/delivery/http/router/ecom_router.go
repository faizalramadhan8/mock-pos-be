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
}

// UseEcomPublicRouter — public storefront endpoints (no auth). Customer
// browse produk sebelum login. Filter enforce ecom_is_available + stock_ecom>0.
func UseEcomPublicRouter(ctx context.Context, r fiber.Router) {
	ctrl := handler.NewEcomPublicController(ctx)
	g := r.Group("/ecom")
	g.Get("/categories", ctrl.ListCategories)
	g.Get("/products", ctrl.ListProducts)
	g.Get("/products/:id", ctrl.GetProduct)
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
	// Customer own orders
	g.Get("/orders", ordersCtrl.List)
	g.Get("/orders/:id", ordersCtrl.GetDetail)

	// Midtrans webhook — PUBLIC (Midtrans server hits this, no JWT).
	// Mounted di r (root) — bukan grup /ecom gated.
	r.Post("/ecom/payments/webhook/midtrans", checkoutCtrl.MidtransWebhook)
}
