package handler

import (
	"context"
	"strings"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type DeviceController struct {
	Log     *zerolog.Logger
	Service *usecase.DeviceService
	Configs *config.Config
}

// extractBaseURL derives the public-facing base URL from the incoming request.
// Works for the common setups in this app: direct access, nginx reverse proxy,
// and Cloudflare Tunnel (which sets the CF-Visitor header to signal https even
// when the tunnel leg to origin is plain http).
func extractBaseURL(c *fiber.Ctx) string {
	host := c.Hostname()
	if host == "" {
		return ""
	}
	if cv := c.Get("CF-Visitor"); strings.Contains(cv, `"scheme":"https"`) {
		return "https://" + host
	}
	if proto := c.Get("X-Forwarded-Proto"); proto != "" {
		return proto + "://" + host
	}
	return c.Protocol() + "://" + host
}

func NewDeviceController(ctx context.Context) *DeviceController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &DeviceController{
		Log:     logger,
		Service: usecase.NewDeviceService(ctx, db),
		Configs: configs,
	}
}

// GetStatus is polled by the frontend while waiting for owner approval.
// Query params: email (to resolve user_id without requiring auth) + fingerprint.
func (ctrl *DeviceController) GetStatus(c *fiber.Ctx) error {
	email := strings.TrimSpace(c.Query("email"))
	fingerprint := strings.TrimSpace(c.Query("fingerprint"))
	if email == "" || fingerprint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.StatusBadRequest,
			Message: "email and fingerprint are required",
		})
	}
	user, err := ctrl.Service.AuthR.FindByEmail(email)
	if err != nil {
		return c.JSON(dto.ApiResponse{
			Code:    fiber.StatusOK,
			Message: "successfully",
			Body:    dto.DeviceStatusResponse{Status: "unknown", Fingerprint: fingerprint},
		})
	}
	resp, fail := ctrl.Service.GetStatus(user.ID, fingerprint)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// List devices for a given user (admin only).
func (ctrl *DeviceController) List(c *fiber.Ctx) error {
	userID := c.Params("id")
	devices, fail := ctrl.Service.ListByUser(userID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: devices})
}

// Revoke deletes a trusted device record (admin only).
func (ctrl *DeviceController) Revoke(c *fiber.Ctx) error {
	deviceID := c.Params("device_id")
	if fail := ctrl.Service.Revoke(deviceID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Device revoked"})
}

// EmergencyApprove is the manual fallback when WA links don't work
// (superadmin only).
func (ctrl *DeviceController) EmergencyApprove(c *fiber.Ctx) error {
	deviceID := c.Params("device_id")
	if fail := ctrl.Service.EmergencyApprove(deviceID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Device approved"})
}

// deviceActionResponse is returned to the FE approval page after an
// approve/reject token submission.
type deviceActionResponse struct {
	Status   string `json:"status"` // "approved" | "rejected"
	UserName string `json:"user_name"`
}

// Approve is called by the FE approval page with ?t=<token>. Returns JSON.
func (ctrl *DeviceController) Approve(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Query("t"))
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.StatusBadRequest,
			Message: "token kosong",
			Error:   "token is required",
		})
	}
	_, user, err := ctrl.Service.ApproveByCode(token)
	if err != nil {
		ctrl.Log.Warn().Err(err).Msg("approve via link failed")
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
			Error:   err.Error(),
		})
	}
	ctrl.Service.SendConfirmation(user, true)
	name := "Kasir"
	if user != nil {
		name = user.FullName
	}
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    deviceActionResponse{Status: "approved", UserName: name},
	})
}

// Reject is called by the FE approval page with ?t=<token>. Returns JSON.
func (ctrl *DeviceController) Reject(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Query("t"))
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.StatusBadRequest,
			Message: "token kosong",
			Error:   "token is required",
		})
	}
	_, user, err := ctrl.Service.RejectByCode(token)
	if err != nil {
		ctrl.Log.Warn().Err(err).Msg("reject via link failed")
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.StatusBadRequest,
			Message: err.Error(),
			Error:   err.Error(),
		})
	}
	ctrl.Service.SendConfirmation(user, false)
	name := "Kasir"
	if user != nil {
		name = user.FullName
	}
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    deviceActionResponse{Status: "rejected", UserName: name},
	})
}
