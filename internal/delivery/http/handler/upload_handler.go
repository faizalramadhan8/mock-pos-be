package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type UploadController struct {
	Log *zerolog.Logger
}

func NewUploadController(ctx context.Context) *UploadController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &UploadController{
		Log: logger,
	}
}

func (ctrl *UploadController) UploadImage(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: "File is required",
			Error:   err.Error(),
		})
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowedExts[ext] {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: "Only image files (jpg, jpeg, png, gif, webp) are allowed",
		})
	}

	// Validate file size (max 5MB)
	if file.Size > 5*1024*1024 {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.ErrBadRequest.Code,
			Message: "File size must be less than 5MB",
		})
	}

	// Determine upload directory based on type query param
	uploadType := c.Query("type", "products")
	var uploadDir string
	switch uploadType {
	case "products":
		uploadDir = "storage/products"
	case "payment-proof":
		uploadDir = "storage/payment-proof"
	default:
		uploadDir = "storage/uploads"
	}

	// Create directory if not exists
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		ctrl.Log.Error().Err(err).Msg("Failed to create upload directory")
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ApiResponse{
			Code:    fiber.ErrInternalServerError.Code,
			Message: "Failed to create upload directory",
		})
	}

	// Generate unique filename
	filename := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102150405"), uuid.New().String()[:8], ext)
	savePath := filepath.Join(uploadDir, filename)

	// Save file
	if err := c.SaveFile(file, savePath); err != nil {
		ctrl.Log.Error().Err(err).Msg("Failed to save file")
		return c.Status(fiber.StatusInternalServerError).JSON(dto.ApiResponse{
			Code:    fiber.ErrInternalServerError.Code,
			Message: "Failed to save file",
		})
	}

	// Return the URL path (relative to storage root)
	urlPath := "/" + savePath

	ctrl.Log.Info().Msgf("File uploaded: %s", savePath)

	return c.JSON(dto.ApiResponse{
		Code:    fiber.StatusOK,
		Message: "File uploaded successfully",
		Body: map[string]string{
			"url":      urlPath,
			"filename": filename,
		},
	})
}
