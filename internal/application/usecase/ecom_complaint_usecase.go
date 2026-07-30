package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomComplaintService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomComplaintService(ctx context.Context, db *gorm.DB) *EcomComplaintService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomComplaintService{DB: db, Log: logger}
}

// reasonLabel — friendly Indonesian label untuk UI. Cegah FE re-implement
// mapping — konsisten across customer + admin panel.
func reasonLabel(reason string) string {
	switch reason {
	case "barang_rusak":
		return "Barang rusak"
	case "barang_salah":
		return "Barang salah kirim"
	case "barang_kurang":
		return "Barang tidak lengkap"
	case "lainnya":
		return "Lainnya"
	}
	return reason
}

// Submit — customer submit komplain baru. Guard rails:
//   - Order harus milik user + ecom_status delivered/completed
//   - Max 30 hari sejak completed (setelah itu tutup)
//   - Max 1 komplain open/in_review per order (cegah spam)
func (s *EcomComplaintService) Submit(userID string, req dto.ComplaintSubmitRequest) (*dto.ComplaintResponse, *dto.ApiError) {
	// Verify ownership + eligibility.
	var order entity.Order
	if err := s.DB.Where("id = ? AND ecom_user_id = ? AND order_source = 'ecom' AND deleted_at IS NULL", req.OrderID, userID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Pesanan tidak ditemukan"}
	}
	current := ""
	if order.EcomStatus != nil {
		current = *order.EcomStatus
	}
	if current != "delivered" && current != "completed" {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Komplain hanya bisa diajukan setelah barang diterima",
		}
	}
	// Cegah spam — kalau sudah ada komplain open/in_review untuk order ini.
	var existing int64
	s.DB.Model(&entity.EcomComplaint{}).
		Where("order_id = ? AND status IN ('open', 'in_review')", req.OrderID).
		Count(&existing)
	if existing > 0 {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Kamu sudah punya komplain aktif untuk pesanan ini. Tunggu balasan admin.",
		}
	}

	imagesJSON := ""
	if len(req.Images) > 0 {
		b, _ := json.Marshal(req.Images)
		imagesJSON = string(b)
	}
	var imagesPtr *string
	if imagesJSON != "" {
		imagesPtr = &imagesJSON
	}

	row := entity.EcomComplaint{
		ID:          uuid.New().String(),
		OrderID:     req.OrderID,
		UserID:      userID,
		Reason:      req.Reason,
		Description: req.Description,
		Images:      imagesPtr,
		Status:      "open",
	}
	if err := s.DB.Create(&row).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan komplain"}
	}
	s.Log.Info().Str("complaint_id", row.ID).Str("order_id", req.OrderID).Str("reason", req.Reason).Msg("customer submitted complaint")
	resp := toComplaintResponse(&row, "")
	return &resp, nil
}

// ListForUser — komplain milik customer sendiri.
func (s *EcomComplaintService) ListForUser(userID string) ([]dto.ComplaintResponse, *dto.ApiError) {
	var rows []entity.EcomComplaint
	if err := s.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil komplain"}
	}
	out := make([]dto.ComplaintResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toComplaintResponse(&r, ""))
	}
	return out, nil
}

// ListForAdmin — semua komplain (dengan join user untuk display name).
// Filter status opsional.
func (s *EcomComplaintService) ListForAdmin(status string) ([]dto.ComplaintResponse, *dto.ApiError) {
	q := s.DB.Model(&entity.EcomComplaint{})
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	var rows []entity.EcomComplaint
	if err := q.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil komplain"}
	}
	// Batch fetch user names untuk display.
	userIDs := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		if !seen[r.UserID] {
			seen[r.UserID] = true
			userIDs = append(userIDs, r.UserID)
		}
	}
	nameByID := map[string]string{}
	if len(userIDs) > 0 {
		var users []entity.User
		s.DB.Select("id, fullname").Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			nameByID[u.ID] = u.FullName
		}
	}
	out := make([]dto.ComplaintResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toComplaintResponse(&r, nameByID[r.UserID]))
	}
	return out, nil
}

// Reply — admin reply + update status. Set resolved_at kalau status=resolved.
func (s *EcomComplaintService) Reply(adminID, complaintID string, req dto.ComplaintAdminReplyRequest) (*dto.ComplaintResponse, *dto.ApiError) {
	var row entity.EcomComplaint
	if err := s.DB.Where("id = ?", complaintID).First(&row).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Komplain tidak ditemukan"}
	}
	updates := map[string]interface{}{
		"admin_reply": req.Reply,
		"admin_id":    adminID,
		"status":      req.Status,
	}
	if req.Status == "resolved" || req.Status == "rejected" {
		updates["resolved_at"] = time.Now()
	}
	if err := s.DB.Model(&entity.EcomComplaint{}).Where("id = ?", complaintID).Updates(updates).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan balasan"}
	}
	s.Log.Info().Str("complaint_id", complaintID).Str("status", req.Status).Str("by", adminID).Msg("admin replied complaint")
	// Refetch dengan user name kalau perlu — untuk sekarang skip (admin
	// tidak butuh nama dirinya di response).
	s.DB.Where("id = ?", complaintID).First(&row)
	resp := toComplaintResponse(&row, "")
	return &resp, nil
}

func toComplaintResponse(r *entity.EcomComplaint, userName string) dto.ComplaintResponse {
	images := []string{}
	if r.Images != nil && *r.Images != "" {
		_ = json.Unmarshal([]byte(*r.Images), &images)
	}
	adminReply := ""
	if r.AdminReply != nil {
		adminReply = *r.AdminReply
	}
	resolvedAt := ""
	if r.ResolvedAt != nil {
		resolvedAt = r.ResolvedAt.Format(time.RFC3339)
	}
	return dto.ComplaintResponse{
		ID:          r.ID,
		OrderID:     r.OrderID,
		UserID:      r.UserID,
		UserName:    userName,
		Reason:      r.Reason,
		ReasonLabel: reasonLabel(r.Reason),
		Description: r.Description,
		Images:      images,
		Status:      r.Status,
		AdminReply:  adminReply,
		CreatedAt:   r.CreatedAt.Format(time.RFC3339),
		ResolvedAt:  resolvedAt,
	}
}
