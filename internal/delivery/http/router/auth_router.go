package router

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/handler"
	"github.com/faizalramadhan/pos-be/internal/delivery/http/middleware"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
)

func UseAuthRouter(ctx context.Context, r fiber.Router) {

	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	auth := middleware.NewRBACMiddleware(configs.JwtSecret, configs.JwtAccessTokenExpiresIn)
	ctrl := handler.NewAuthController(ctx)
	deviceCtrl := handler.NewDeviceController(ctx)

	authGroup := r.Group("/auth")
	authGroup.Post("/register", ctrl.Register)
	// Public customer register — force role='user'. Bu Santi 21 Jul 2026.
	authGroup.Post("/register-customer", ctrl.RegisterCustomer)
	authGroup.Post("/login", ctrl.Login)
	authGroup.Post("/logout", ctrl.Logout)
	authGroup.Get("/session", auth.AllowAll(), ctrl.GetProfile)
	authGroup.Post("/change-password", auth.AllowAll(), ctrl.ChangePassword)

	// Customer self-service (Sprint 2) — auth required, self-scoped (user
	// ID diambil dari JWT, tidak dari URL).
	authGroup.Patch("/me", auth.AllowAll(), ctrl.UpdateProfile)
	authGroup.Post("/me/password", auth.AllowAll(), ctrl.ChangeMyPassword)

	// Password reset via email OTP (public — customer lupa password).
	authGroup.Post("/password-reset/request", ctrl.RequestPasswordResetOTP)
	authGroup.Post("/password-reset/confirm", ctrl.ConfirmPasswordResetOTP)

	// Device binding: status polling is public (no token yet — user is still
	// trying to login). Approve/reject links are tapped by owner from
	// WhatsApp; auth is via the single-use token embedded in the URL.
	authGroup.Get("/devices/status", deviceCtrl.GetStatus)
	authGroup.Get("/devices/approve", deviceCtrl.Approve)
	authGroup.Get("/devices/reject", deviceCtrl.Reject)

	users := r.Group("/users", auth.AllowAdmins())
	users.Get("/", ctrl.GetAllUsers)
	users.Post("/", ctrl.Register)
	users.Put("/:id", ctrl.UpdateUser)
	users.Patch("/:id/toggle-active", ctrl.ToggleUserActive)
	users.Post("/:id/reset-password", ctrl.ResetPassword)
	users.Delete("/:id", ctrl.DeleteUser)
	users.Get("/:id/devices", deviceCtrl.List)
	users.Delete("/:id/devices/:device_id", deviceCtrl.Revoke)
	users.Post("/:id/devices/:device_id/approve", deviceCtrl.EmergencyApprove)
}
