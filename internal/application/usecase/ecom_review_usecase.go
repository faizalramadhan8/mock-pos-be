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

type EcomReviewService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomReviewService(ctx context.Context, db *gorm.DB) *EcomReviewService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomReviewService{DB: db, Log: logger}
}

// ListForProduct — public, tampil di PDP. Skip is_hidden. Order rating DESC lalu newest.
func (s *EcomReviewService) ListForProduct(productID string, limit int) ([]dto.ReviewPublicItem, *dto.ReviewSummary, *dto.ApiError) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var reviews []entity.EcomReview
	if err := s.DB.Preload("User").
		Where("product_id = ? AND is_hidden = 0", productID).
		Order("created_at DESC").
		Limit(limit).
		Find(&reviews).Error; err != nil {
		return nil, nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch reviews"}
	}

	items := make([]dto.ReviewPublicItem, 0, len(reviews))
	for _, r := range reviews {
		name := "Customer"
		if r.User != nil && r.User.FullName != "" {
			// Privacy — mask nama: "Faisal R" instead of "Faisal Ramadhan".
			name = maskName(r.User.FullName)
		}
		items = append(items, dto.ReviewPublicItem{
			ID:        r.ID,
			Rating:    r.Rating,
			Comment:   r.Comment,
			UserName:  name,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}

	// Aggregate stats — hitung fresh (small volume, tidak perlu materialized).
	var stat struct {
		Count int
		Avg   float64
	}
	s.DB.Model(&entity.EcomReview{}).
		Select("COUNT(*) as count, COALESCE(AVG(rating), 0) as avg").
		Where("product_id = ? AND is_hidden = 0", productID).
		Scan(&stat)

	// Distribution 1..5.
	var dist []struct {
		Rating int
		Cnt    int
	}
	s.DB.Model(&entity.EcomReview{}).
		Select("rating, COUNT(*) as cnt").
		Where("product_id = ? AND is_hidden = 0", productID).
		Group("rating").Scan(&dist)
	distMap := map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}
	for _, d := range dist {
		distMap[d.Rating] = d.Cnt
	}

	summary := &dto.ReviewSummary{
		Count:        stat.Count,
		Average:      stat.Avg,
		Distribution: distMap,
	}
	return items, summary, nil
}

// CanReview — check apakah user boleh review produk ini. True kalau user
// punya minimal 1 ecom order dengan ecom_status='completed' yang mengandung
// product ini.
//
// Pakai ecom_status (bukan orders.status POS-side) supaya gate baru berlaku:
// customer HARUS konfirmasi barang diterima dulu → status jadi completed →
// baru bisa review. Cegah review palsu dari kurir yang salah tag delivered.
func (s *EcomReviewService) CanReview(userID, productID string) (bool, *dto.ApiError) {
	var count int64
	err := s.DB.Table("order_items oi").
		Joins("JOIN orders o ON o.id = oi.order_id").
		Where("o.ecom_user_id = ? AND o.ecom_status = 'completed' AND o.deleted_at IS NULL AND oi.product_id = ?", userID, productID).
		Count(&count).Error
	if err != nil {
		return false, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to check eligibility"}
	}
	return count > 0, nil
}

// Upsert — insert atau update review by (user_id, product_id).
func (s *EcomReviewService) Upsert(userID string, req dto.ReviewSubmitRequest) (*entity.EcomReview, *dto.ApiError) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Rating harus 1-5"}
	}
	// Eligibility check.
	ok, ferr := s.CanReview(userID, req.ProductID)
	if ferr != nil {
		return nil, ferr
	}
	if !ok {
		return nil, &dto.ApiError{StatusCode: fiber.ErrForbidden, Message: "Kamu belum pernah beli produk ini"}
	}

	// Cari existing.
	var existing entity.EcomReview
	found := s.DB.Where("user_id = ? AND product_id = ?", userID, req.ProductID).First(&existing).Error == nil

	comment := strings.TrimSpace(req.Comment)
	if found {
		existing.Rating = req.Rating
		existing.Comment = comment
		if err := s.DB.Save(&existing).Error; err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save review"}
		}
		return &existing, nil
	}
	newReview := entity.EcomReview{
		ID:        uuid.New().String(),
		ProductID: req.ProductID,
		UserID:    userID,
		Rating:    req.Rating,
		Comment:   comment,
	}
	if err := s.DB.Create(&newReview).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create review"}
	}
	return &newReview, nil
}

// ─── Admin moderation (Sprint 3 #14) ─────────────────────────────────

// ListForAdmin — semua review dengan filter opsional (hidden only, product_id).
// Include user + product info untuk display. Order by newest first.
func (s *EcomReviewService) ListForAdmin(hiddenOnly bool, productID string) ([]dto.ReviewAdminItem, *dto.ApiError) {
	q := s.DB.Model(&entity.EcomReview{}).Preload("User")
	if hiddenOnly {
		q = q.Where("is_hidden = 1")
	}
	if productID != "" {
		q = q.Where("product_id = ?", productID)
	}
	var reviews []entity.EcomReview
	if err := q.Order("created_at DESC").Limit(200).Find(&reviews).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil review"}
	}

	// Batch fetch product names.
	productIDs := make([]string, 0, len(reviews))
	seen := map[string]bool{}
	for _, r := range reviews {
		if !seen[r.ProductID] {
			seen[r.ProductID] = true
			productIDs = append(productIDs, r.ProductID)
		}
	}
	nameByID := map[string]string{}
	if len(productIDs) > 0 {
		var products []entity.Product
		s.DB.Select("id, name_id").Where("id IN ?", productIDs).Find(&products)
		for _, p := range products {
			nameByID[p.ID] = p.NameID
		}
	}

	out := make([]dto.ReviewAdminItem, 0, len(reviews))
	for _, r := range reviews {
		userName := ""
		userEmail := ""
		if r.User != nil {
			userName = r.User.FullName
			userEmail = r.User.Email
		}
		out = append(out, dto.ReviewAdminItem{
			ID:          r.ID,
			ProductID:   r.ProductID,
			ProductName: nameByID[r.ProductID],
			UserID:      r.UserID,
			UserName:    userName,
			UserEmail:   userEmail,
			Rating:      r.Rating,
			Comment:     r.Comment,
			IsHidden:    r.IsHidden,
			CreatedAt:   r.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ToggleHide — admin set is_hidden true/false. Review yang di-hide tidak
// muncul di PDP public tapi tetap tersimpan (soft-hide, bukan delete).
func (s *EcomReviewService) ToggleHide(reviewID string, hidden bool) *dto.ApiError {
	res := s.DB.Model(&entity.EcomReview{}).Where("id = ?", reviewID).Update("is_hidden", hidden)
	if res.Error != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal update review"}
	}
	if res.RowsAffected == 0 {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Review tidak ditemukan"}
	}
	s.Log.Info().Str("review_id", reviewID).Bool("hidden", hidden).Msg("admin toggled review visibility")
	return nil
}

// GetMyReview — cek existing review user untuk produk ini. Dipakai FE untuk
// prefill form / display "Kamu sudah review" state.
func (s *EcomReviewService) GetMyReview(userID, productID string) (*entity.EcomReview, *dto.ApiError) {
	var r entity.EcomReview
	err := s.DB.Where("user_id = ? AND product_id = ?", userID, productID).First(&r).Error
	if err != nil {
		return nil, nil // not found = tidak error, cukup return nil review
	}
	return &r, nil
}

// maskName — Privacy. "Faisal Ramadhan" → "Faisal R.". First word full, last
// word initial + dot. Kalau cuma 1 kata, biarkan (mis. "Aura").
func maskName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Customer"
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0]
	}
	last := parts[len(parts)-1]
	initial := "?"
	if len(last) > 0 {
		initial = string([]rune(last)[0])
	}
	return parts[0] + " " + strings.ToUpper(initial) + "."
}
