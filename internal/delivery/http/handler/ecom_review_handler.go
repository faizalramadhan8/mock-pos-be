package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomReviewController struct {
	Log *zerolog.Logger
	Svc *usecase.EcomReviewService
}

func NewEcomReviewController(ctx context.Context) *EcomReviewController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomReviewController{Log: logger, Svc: usecase.NewEcomReviewService(ctx, db)}
}

// GET /ecom/products/:id/reviews — public.
func (ctrl *EcomReviewController) ListForProduct(c *fiber.Ctx) error {
	productID := c.Params("id")
	limit := c.QueryInt("limit", 20)
	items, summary, fail := ctrl.Svc.ListForProduct(productID, limit)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code: fail.StatusCode.Code, Message: fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: dto.ReviewListResponse{
		Items: items, Summary: *summary,
	}})
}

// GET /ecom/products/:id/reviews/me — auth. Check eligibility + fetch existing review.
func (ctrl *EcomReviewController) CanReviewMe(c *fiber.Ctx) error {
	productID := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	ok, fail := ctrl.Svc.CanReview(claims.ID, productID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	existing, _ := ctrl.Svc.GetMyReview(claims.ID, productID)
	resp := dto.ReviewCanReviewResponse{CanReview: ok}
	if existing != nil {
		resp.MyReview = &dto.ReviewPublicItem{
			ID:        existing.ID,
			Rating:    existing.Rating,
			Comment:   existing.Comment,
			UserName:  "You",
			CreatedAt: existing.CreatedAt.Format(time.RFC3339),
		}
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: resp})
}

// POST /ecom/reviews — customer submit atau update.
func (ctrl *EcomReviewController) Submit(c *fiber.Ctx) error {
	var req dto.ReviewSubmitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{
			Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body", Error: err.Error(),
		})
	}
	if err := util.ValidateRequest(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Invalid input", Error: err,
		})
	}
	claims := c.Locals("session").(*dto.JWTClaims)
	r, fail := ctrl.Svc.Upsert(claims.ID, req)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Review tersimpan", Body: r})
}

// ─── Admin moderation (Sprint 3 #14) ─────────────────────────────────

// ListForAdmin — admin lihat semua review, filter opsional hidden_only + product.
func (ctrl *EcomReviewController) ListForAdmin(c *fiber.Ctx) error {
	hiddenOnly := c.Query("hidden_only") == "1" || c.Query("hidden_only") == "true"
	productID := c.Query("product_id")
	items, fail := ctrl.Svc.ListForAdmin(hiddenOnly, productID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "OK", Body: items})
}

// ToggleHide — PATCH /admin/reviews/:id — admin hide/unhide review.
func (ctrl *EcomReviewController) ToggleHide(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.ReviewToggleHideRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(dto.ApiResponse{Code: fiber.ErrUnprocessableEntity.Code, Message: "Invalid body"})
	}
	if fail := ctrl.Svc.ToggleHide(id, req.IsHidden); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.Message})
	}
	msg := "Review disembunyikan"
	if !req.IsHidden {
		msg = "Review ditampilkan kembali"
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: msg})
}
