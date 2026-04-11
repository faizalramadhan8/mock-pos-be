package handler

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type AuthController struct {
	Log         *zerolog.Logger
	AuthService *usecase.AuthService
}

func NewAuthController(ctx context.Context) *AuthController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &AuthController{
		Log:         logger,
		AuthService: usecase.NewAuthService(ctx, db),
	}
}

func (ctrl *AuthController) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code:    fiber.ErrUnprocessableEntity.Code,
			Message: fiber.ErrUnprocessableEntity.Message,
			Error:   err.Error(),
		})
	}

	ctrl.Log.Info().Msg(fmt.Sprintf("User register: %v", util.FormattedJson(req)))

	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: fiber.ErrBadRequest.Message,
			Error:   err,
		})
	}

	resp, fail := ctrl.AuthService.Register(req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	ctrl.Log.Info().Msg("successfuly register user")

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    resp,
	})
}

func (ctrl *AuthController) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code:    fiber.ErrUnprocessableEntity.Code,
			Message: fiber.ErrUnprocessableEntity.Message,
			Error:   err.Error(),
		})
	}

	ctrl.Log.Info().Msg(fmt.Sprintf("User login: %v", util.FormattedJson(req)))

	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: fiber.ErrBadRequest.Message,
			Error:   err,
		})
	}

	resp, fail := ctrl.AuthService.Login(req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	ctrl.Log.Info().Msg("successfuly login user")
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    resp,
	})
}

func (ctrl *AuthController) GetProfile(c *fiber.Ctx) error {
	claims := c.Locals("session").(*dto.JWTClaims)

	profile, fail := ctrl.AuthService.GetSession(claims)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    profile,
	})
}

func (ctrl *AuthController) RefreshToken(c *fiber.Ctx) error {
	var req dto.RefreshTokenRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code:    fiber.ErrUnprocessableEntity.Code,
			Message: fiber.ErrUnprocessableEntity.Message,
			Error:   err.Error(),
		})
	}

	ctrl.Log.Info().Msg("Refreshing access token")

	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: fiber.ErrBadRequest.Message,
			Error:   err,
		})
	}

	resp, fail := ctrl.AuthService.RefreshToken(req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	ctrl.Log.Info().Msg("Token refreshed successfully")
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    resp,
	})
}

func (ctrl *AuthController) Logout(c *fiber.Ctx) error {
	var req dto.RefreshTokenRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code:    fiber.ErrUnprocessableEntity.Code,
			Message: fiber.ErrUnprocessableEntity.Message,
			Error:   err.Error(),
		})
	}

	ctrl.Log.Info().Msg("User logout")

	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: fiber.ErrBadRequest.Message,
			Error:   err,
		})
	}

	if fail := ctrl.AuthService.Logout(req.RefreshToken); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	ctrl.Log.Info().Msg("User logged out successfully")
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "Logged out successfully",
	})
}

func (ctrl *AuthController) LogoutAll(c *fiber.Ctx) error {
	claims := c.Locals("session").(*dto.JWTClaims)

	ctrl.Log.Info().Msgf("User %s logging out from all devices", claims.ID)

	if fail := ctrl.AuthService.LogoutAll(claims.ID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	ctrl.Log.Info().Msgf("User %s logged out from all devices", claims.ID)
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "Logged out from all devices successfully",
	})
}

func (ctrl *AuthController) GetAllUsers(c *fiber.Ctx) error {
	users, fail := ctrl.AuthService.GetAllUsers()
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    users,
	})
}

func (ctrl *AuthController) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateUserRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code:    fiber.ErrUnprocessableEntity.Code,
			Message: fiber.ErrUnprocessableEntity.Message,
			Error:   err.Error(),
		})
	}

	resp, fail := ctrl.AuthService.UpdateUser(id, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    resp,
	})
}

func (ctrl *AuthController) ToggleUserActive(c *fiber.Ctx) error {
	id := c.Params("id")

	resp, fail := ctrl.AuthService.ToggleUserActive(id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    resp,
	})
}

func (ctrl *AuthController) ResetPassword(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.ResetPasswordRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code:    fiber.ErrUnprocessableEntity.Code,
			Message: fiber.ErrUnprocessableEntity.Message,
			Error:   err.Error(),
		})
	}

	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: fiber.ErrBadRequest.Message,
			Error:   err,
		})
	}

	if fail := ctrl.AuthService.ResetPassword(id, req); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "Password reset successfully",
	})
}

func (ctrl *AuthController) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")

	if fail := ctrl.AuthService.DeleteUser(id); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "User deleted successfully",
	})
}
