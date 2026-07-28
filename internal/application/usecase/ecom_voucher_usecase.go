package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomVoucherService — logic voucher/promo code untuk ecom checkout.
// Sprint 5, Bu Santi 28 Jul 2026.
type EcomVoucherService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomVoucherService(ctx context.Context, db *gorm.DB) *EcomVoucherService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomVoucherService{DB: db, Log: logger}
}

// Validate — customer input kode + subtotal cart, kita return computed
// discount amount atau error message. Ini query-only, TIDAK increment
// used_count (increment terjadi saat checkout success — di CreateOrder).
func (s *EcomVoucherService) Validate(code string, subtotal float64) (*dto.VoucherValidateResponse, *dto.ApiError) {
	normalized := normalizeVoucherCode(code)
	if normalized == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Kode voucher kosong"}
	}

	var v entity.EcomVoucher
	if err := s.DB.Where("code = ? AND deleted_at IS NULL", normalized).First(&v).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Kode voucher tidak ditemukan"}
	}
	if !v.IsActive {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher tidak aktif"}
	}
	now := time.Now()
	if v.StartsAt != nil && v.StartsAt.After(now) {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher belum bisa digunakan"}
	}
	if v.ExpiresAt != nil && v.ExpiresAt.Before(now) {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher sudah kadaluarsa"}
	}
	if v.UsageLimit > 0 && v.UsedCount >= v.UsageLimit {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher sudah habis dipakai"}
	}
	if subtotal < v.MinSubtotal {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Minimum belanja Rp " + formatRupiahVoucher(v.MinSubtotal),
		}
	}

	discount := ComputeVoucherDiscount(v, subtotal)
	return &dto.VoucherValidateResponse{
		Code:        v.Code,
		Description: v.Description,
		Type:        v.Type,
		Value:       v.Value,
		Discount:    discount,
	}, nil
}

// ComputeVoucherDiscount — pure function, dipakai di Validate + CreateOrder
// (re-verify server-side, jangan trust FE). Percent-type cap ke MaxDiscount.
// Fixed-type di-clamp ke subtotal (voucher > subtotal = subtotal).
func ComputeVoucherDiscount(v entity.EcomVoucher, subtotal float64) float64 {
	if v.Type == "percent" {
		d := subtotal * (v.Value / 100)
		if v.MaxDiscount != nil && d > *v.MaxDiscount {
			d = *v.MaxDiscount
		}
		return d
	}
	// fixed
	d := v.Value
	if d > subtotal {
		d = subtotal
	}
	return d
}

// ApplyOnCheckout — dipanggil di CreateOrder saat commit tx. Ambil voucher
// row untuk final check (dedup vs race), increment used_count.
// Return error kalau ternyata sudah expired / habis / dst antara Validate
// (di FE) dan Create (di BE) — cegah exploit multi-tab.
func (s *EcomVoucherService) ApplyOnCheckout(tx *gorm.DB, code string, subtotal float64) (float64, *entity.EcomVoucher, *dto.ApiError) {
	normalized := normalizeVoucherCode(code)
	if normalized == "" {
		return 0, nil, nil
	}
	var v entity.EcomVoucher
	// SELECT FOR UPDATE — lock row supaya concurrent checkout dengan voucher
	// yang usage_limit tinggal 1 tidak double-consume.
	if err := tx.Where("code = ? AND deleted_at IS NULL", normalized).First(&v).Error; err != nil {
		return 0, nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Kode voucher tidak valid"}
	}
	// Re-verify semua guard (mirror Validate).
	now := time.Now()
	if !v.IsActive {
		return 0, nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher tidak aktif"}
	}
	if v.StartsAt != nil && v.StartsAt.After(now) {
		return 0, nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher belum bisa digunakan"}
	}
	if v.ExpiresAt != nil && v.ExpiresAt.Before(now) {
		return 0, nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher sudah kadaluarsa"}
	}
	if v.UsageLimit > 0 && v.UsedCount >= v.UsageLimit {
		return 0, nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Voucher sudah habis"}
	}
	if subtotal < v.MinSubtotal {
		return 0, nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Minimum belanja Rp " + formatRupiahVoucher(v.MinSubtotal)}
	}
	discount := ComputeVoucherDiscount(v, subtotal)

	// Increment counter atomic.
	if err := tx.Model(&entity.EcomVoucher{}).
		Where("id = ?", v.ID).
		UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error; err != nil {
		return 0, nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to consume voucher"}
	}

	return discount, &v, nil
}

// ─── Admin CRUD ─────────────────────────────────────────────────────

func (s *EcomVoucherService) List() ([]entity.EcomVoucher, *dto.ApiError) {
	var out []entity.EcomVoucher
	if err := s.DB.Where("deleted_at IS NULL").Order("created_at DESC").Find(&out).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch vouchers"}
	}
	return out, nil
}

func (s *EcomVoucherService) Create(req dto.VoucherCreateRequest) (*entity.EcomVoucher, *dto.ApiError) {
	if err := validateVoucherType(req.Type); err != nil {
		return nil, err
	}
	code := normalizeVoucherCode(req.Code)
	if code == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Kode voucher wajib"}
	}
	// Dedup check.
	var exists int64
	s.DB.Model(&entity.EcomVoucher{}).Where("code = ? AND deleted_at IS NULL", code).Count(&exists)
	if exists > 0 {
		return nil, &dto.ApiError{StatusCode: fiber.ErrConflict, Message: "Kode sudah ada"}
	}
	v := entity.EcomVoucher{
		ID:          uuid.New().String(),
		Code:        code,
		Description: req.Description,
		Type:        req.Type,
		Value:       req.Value,
		MinSubtotal: req.MinSubtotal,
		MaxDiscount: req.MaxDiscount,
		UsageLimit:  req.UsageLimit,
		IsActive:    true,
		StartsAt:    parseTimePtr(req.StartsAt),
		ExpiresAt:   parseTimePtr(req.ExpiresAt),
	}
	if err := s.DB.Create(&v).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create voucher"}
	}
	return &v, nil
}

func (s *EcomVoucherService) Update(id string, req dto.VoucherCreateRequest) (*entity.EcomVoucher, *dto.ApiError) {
	var v entity.EcomVoucher
	if err := s.DB.Where("id = ? AND deleted_at IS NULL", id).First(&v).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Voucher tidak ditemukan"}
	}
	if req.Type != "" {
		if err := validateVoucherType(req.Type); err != nil {
			return nil, err
		}
		v.Type = req.Type
	}
	if req.Description != "" {
		v.Description = req.Description
	}
	if req.Value > 0 {
		v.Value = req.Value
	}
	v.MinSubtotal = req.MinSubtotal
	v.MaxDiscount = req.MaxDiscount
	v.UsageLimit = req.UsageLimit
	v.StartsAt = parseTimePtr(req.StartsAt)
	v.ExpiresAt = parseTimePtr(req.ExpiresAt)
	if req.IsActive != nil {
		v.IsActive = *req.IsActive
	}
	if err := s.DB.Save(&v).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update"}
	}
	return &v, nil
}

func (s *EcomVoucherService) Delete(id string) *dto.ApiError {
	if err := s.DB.Where("id = ?", id).Delete(&entity.EcomVoucher{}).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete"}
	}
	return nil
}

func normalizeVoucherCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func validateVoucherType(t string) *dto.ApiError {
	if t != "percent" && t != "fixed" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Type harus 'percent' atau 'fixed'"}
	}
	return nil
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	// Accept RFC3339 atau YYYY-MM-DD.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t
	}
	return nil
}

// formatRupiahVoucher — sederhana untuk error message. Ga pakai locale package
// supaya no dep. `intl` pattern.
func formatRupiahVoucher(n float64) string {
	s := ""
	i := int64(n)
	if i == 0 {
		return "0"
	}
	for i > 0 {
		r := i % 1000
		i /= 1000
		part := ""
		if i > 0 {
			part = pad3(r)
		} else {
			part = itoa(r)
		}
		if s == "" {
			s = part
		} else {
			s = part + "." + s
		}
	}
	return s
}
func pad3(n int64) string {
	s := itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}
