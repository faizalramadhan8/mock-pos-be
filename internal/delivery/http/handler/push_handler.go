package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type PushController struct {
	Log     *zerolog.Logger
	Service *usecase.PushService
}

func NewPushController(ctx context.Context) *PushController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &PushController{
		Log:     logger,
		Service: usecase.NewPushService(ctx, db),
	}
}

func (ctrl *PushController) GetVAPIDKey(c *fiber.Ctx) error {
	key := ctrl.Service.GetVAPIDPublicKey()
	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "successfully",
		Body:    dto.VAPIDKeysResponse{PublicKey: key},
	})
}

func (ctrl *PushController) Subscribe(c *fiber.Ctx) error {
	var req dto.SubscribePushRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err,
		})
	}

	claims := c.Locals("session").(*dto.JWTClaims)
	if fail := ctrl.Service.Subscribe(req, claims.ID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}

	return c.Status(fiber.StatusCreated).JSON(dto.ApiResponse{
		Code: fiber.StatusCreated, Message: "Subscribed to push notifications",
	})
}

func (ctrl *PushController) Unsubscribe(c *fiber.Ctx) error {
	var req dto.UnsubscribePushRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: fiber.ErrUnprocessableEntity.Message, Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: fiber.ErrBadRequest.Message, Error: err,
		})
	}

	if fail := ctrl.Service.Unsubscribe(req); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message,
		})
	}

	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Unsubscribed"})
}
