package dto

import "github.com/gofiber/fiber/v2"

type ApiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   any    `json:"error,omitempty"`
	Body    any    `json:"body,omitempty"`
	Meta    any    `json:"meta,omitempty"`
}

type ApiFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ApiError struct {
	StatusCode *fiber.Error
	Message    string
}
