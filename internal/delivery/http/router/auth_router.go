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

	authGroup := r.Group("/auth")
	authGroup.Post("/register", ctrl.Register)
	authGroup.Post("/login", ctrl.Login)
	authGroup.Post("/refresh", ctrl.RefreshToken)
	authGroup.Post("/logout", ctrl.Logout)
	authGroup.Get("/session", auth.AllowAll(), ctrl.GetProfile)
	authGroup.Post("/change-password", auth.AllowAll(), ctrl.ChangePassword)
	authGroup.Post("/logout-all", auth.AllowSuperAdmin(), ctrl.LogoutAll)

	users := r.Group("/users", auth.AllowAdmins())
	users.Get("/", ctrl.GetAllUsers)
	users.Post("/", ctrl.Register)
	users.Put("/:id", ctrl.UpdateUser)
	users.Patch("/:id/toggle-active", ctrl.ToggleUserActive)
	users.Post("/:id/reset-password", ctrl.ResetPassword)
	users.Delete("/:id", ctrl.DeleteUser)
}
