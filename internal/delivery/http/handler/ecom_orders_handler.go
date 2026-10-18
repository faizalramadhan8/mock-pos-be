package handler

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomOrdersController struct {
	Log     *zerolog.Logger
	Service *usecase.EcomOrdersService
}

func NewEcomOrdersController(ctx context.Context) *EcomOrdersController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &EcomOrdersController{
		Log:     logger,
		Service: usecase.NewEcomOrdersService(ctx, db),
	}
}

func (ctrl *EcomOrdersController) List(c *fiber.Ctx) error {
	claims := c.Locals("session").(*dto.JWTClaims)
	rows, fail := ctrl.Service.List(claims.ID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: rows})
}

func (ctrl *EcomOrdersController) GetDetail(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.GetDetail(claims.ID, id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// GetTracking — riwayat perjalanan paket milik customer sendiri.
//
// Body `null` berarti resinya belum terbit — bukan kesalahan. Nomor resi baru
// turun lewat webhook setelah kurir mengalokasikan, jadi jeda antara pesanan
// dibuat dan resi muncul itu normal. FE menampilkannya sebagai keterangan,
// bukan layar error.
func (ctrl *EcomOrdersController) GetTracking(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.GetTracking(claims.ID, id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// ConfirmReceived — customer klik "Barang Diterima". Body kosong — cukup
// order id di URL + JWT untuk ownership check di service.
func (ctrl *EcomOrdersController) ConfirmReceived(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.ConfirmReceived(claims.ID, id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Terima kasih! Pesanan ditandai selesai.", Body: resp})
}

// CancelPending — customer batalkan sendiri order yang masih pending_payment.
// Restore stok otomatis, set ecom_status='cancelled'.
func (ctrl *EcomOrdersController) CancelPending(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.CancelPending(claims.ID, id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Pesanan dibatalkan", Body: resp})
}

// SubmitPaymentProof — customer kirim bukti transfer untuk pesanannya.
// Body: {"payment_proof": "/storage/payment-proof/xxx.jpg"} — URL dari
// POST /upload?type=payment-proof.
func (ctrl *EcomOrdersController) SubmitPaymentProof(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	var req struct {
		PaymentProof string `json:"payment_proof"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code: fiber.ErrBadRequest.Code, Message: "Format permintaan tidak valid", Error: err.Error()})
	}
	resp, fail := ctrl.Service.SubmitPaymentProof(claims.ID, id, req.PaymentProof)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Bukti transfer terkirim", Body: resp})
}

// RetryPayment — regenerate payment_url PG untuk order pending_payment yang
// link-nya expired. Auto-sync status kalau ternyata PG bilang sudah PAID.
func (ctrl *EcomOrdersController) RetryPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	claims := c.Locals("session").(*dto.JWTClaims)
	resp, fail := ctrl.Service.RetryPayment(claims.ID, id)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{Code: fail.StatusCode.Code, Message: fail.StatusCode.Message, Error: fail.Message})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Link bayar diperbarui", Body: resp})
}
