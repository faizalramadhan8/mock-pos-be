package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminService — usecase untuk admin panel ecom. Strict separation:
// TIDAK sentuh field POS (stock, selling_price, member_price, name, dst).
// Cegah admin ecom accidentally break POS pricing. Bu Santi 20 Jul 2026.
type EcomAdminService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAdminService(ctx context.Context, db *gorm.DB) *EcomAdminService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminService{DB: db, Log: logger}
}

// ListProducts — cursor pagination by created_at DESC. Search LIKE match ke
// name / sku. Return SEMUA produk (termasuk yang stock_ecom=0 atau
// ecom_is_available=FALSE) supaya admin bisa toggle publish + set stok.
func (s *EcomAdminService) ListProducts(search, cursor string, limit int) ([]dto.EcomAdminProductResponse, string, *dto.ApiError) {
	var products []entity.Product

	q := s.DB.Model(&entity.Product{}).Where("deleted_at IS NULL")

	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ? OR name_id LIKE ? OR sku LIKE ?", like, like, like)
	}

	if cursor != "" {
		if t, err := time.Parse(time.RFC3339, cursor); err == nil {
			q = q.Where("created_at < ?", t)
		}
	}

	if err := q.Order("created_at DESC").Limit(limit).Find(&products).Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to list ecom products")
		return nil, "", &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch products"}
	}

	items := make([]dto.EcomAdminProductResponse, 0, len(products))
	for i := range products {
		items = append(items, toEcomAdminProductResponse(&products[i]))
	}

	nextCursor := ""
	if len(products) == limit && limit > 0 {
		nextCursor = products[len(products)-1].CreatedAt.Format(time.RFC3339)
	}

	return items, nextCursor, nil
}

// GetProduct — detail single untuk admin edit page.
func (s *EcomAdminService) GetProduct(id string) (*dto.EcomAdminProductResponse, *dto.ApiError) {
	var product entity.Product
	if err := s.DB.Preload("EcomCategory").Where("id = ? AND deleted_at IS NULL", id).First(&product).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	resp := toEcomAdminProductResponse(&product)
	return &resp, nil
}

// BulkUpdateResult — Sprint 5 Chunk 10.
type BulkUpdateResult struct {
	AffectedCount int      `json:"affected_count"`
	SkippedCount  int      `json:"skipped_count"`
	SkippedIDs    []string `json:"skipped_ids,omitempty"`
}

// BulkSetAvailable — publish/unpublish batch. Set ecom_is_available untuk
// SEMUA product yang ID-nya di request. Sprint 5 Chunk 10 (2 Aug 2026).
// Skala Bu Santi kecil (< 500 products), single UPDATE dalam 1 query cukup.
func (s *EcomAdminService) BulkSetAvailable(ids []string, available bool) (*BulkUpdateResult, *dto.ApiError) {
	if len(ids) == 0 {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Pilih minimal 1 produk"}
	}
	res := s.DB.Model(&entity.Product{}).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Update("ecom_is_available", available)
	if res.Error != nil {
		s.Log.Error().Err(res.Error).Msg("Bulk set availability failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal update batch"}
	}
	return &BulkUpdateResult{AffectedCount: int(res.RowsAffected)}, nil
}

// BulkSyncPrice — set ecom_price = selling_price untuk batch produk. Kalau
// admin mau harga ecom persis match toko. Skip produk yang selling_price=0
// (invalid). Sprint 5 Chunk 10 (2 Aug 2026).
func (s *EcomAdminService) BulkSyncPrice(ids []string) (*BulkUpdateResult, *dto.ApiError) {
	if len(ids) == 0 {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Pilih minimal 1 produk"}
	}
	// UPDATE dengan expression SET ecom_price = selling_price (per-row).
	// Skip WHERE selling_price > 0 supaya tidak overwrite dengan 0.
	res := s.DB.Exec(
		"UPDATE products SET ecom_price = selling_price WHERE id IN ? AND deleted_at IS NULL AND selling_price > 0",
		ids,
	)
	if res.Error != nil {
		s.Log.Error().Err(res.Error).Msg("Bulk sync price failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal sync harga"}
	}
	return &BulkUpdateResult{AffectedCount: int(res.RowsAffected)}, nil
}

// BulkResetPrice — set ecom_price = NULL supaya fallback ke selling_price
// (behavior default). Sprint 5 Chunk 10 (2 Aug 2026).
func (s *EcomAdminService) BulkResetPrice(ids []string) (*BulkUpdateResult, *dto.ApiError) {
	if len(ids) == 0 {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Pilih minimal 1 produk"}
	}
	res := s.DB.Model(&entity.Product{}).
		Where("id IN ? AND deleted_at IS NULL", ids).
		Update("ecom_price", nil)
	if res.Error != nil {
		s.Log.Error().Err(res.Error).Msg("Bulk reset price failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal reset harga"}
	}
	return &BulkUpdateResult{AffectedCount: int(res.RowsAffected)}, nil
}

// UpdateEcomFields — patch HANYA field ecom_*. Pakai gorm.DB.Model().Updates()
// dengan struct field explicit supaya kolom POS tidak ke-touch. Cegah GORM
// zero-value overwrite (Updates dengan struct skip zero values by default).
func (s *EcomAdminService) UpdateEcomFields(id string, req dto.EcomFieldsUpdateRequest, changedBy string) (*dto.EcomAdminProductResponse, *dto.ApiError) {
	var product entity.Product
	if err := s.DB.Where("id = ? AND deleted_at IS NULL", id).First(&product).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}

	// Build updates map — hanya kolom ecom_* + stock_ecom.
	// Pakai map (bukan struct) supaya explicit control tiap kolom + support
	// setNULL untuk pointer field (kalau user kosongkan input).
	updates := map[string]interface{}{}

	if req.StockEcom != nil {
		if *req.StockEcom < 0 {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Stok online tidak boleh negatif"}
		}
		updates["stock_ecom"] = *req.StockEcom
	}
	if req.EcomIsAvailable != nil {
		updates["ecom_is_available"] = *req.EcomIsAvailable
	}
	if req.EcomPrice != nil {
		if req.EcomPrice.Null {
			updates["ecom_price"] = nil
		} else {
			if req.EcomPrice.Value < 0 {
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Harga online tidak boleh negatif"}
			}
			updates["ecom_price"] = req.EcomPrice.Value
		}
	}
	if req.EcomMemberPrice != nil {
		if req.EcomMemberPrice.Null {
			updates["ecom_member_price"] = nil
		} else {
			if req.EcomMemberPrice.Value < 0 {
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Harga member online tidak boleh negatif"}
			}
			updates["ecom_member_price"] = req.EcomMemberPrice.Value
		}
	}
	if req.EcomWeightGrams != nil {
		if req.EcomWeightGrams.Null {
			updates["ecom_weight_grams"] = nil
		} else {
			if req.EcomWeightGrams.Value < 0 {
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Berat tidak boleh negatif"}
			}
			updates["ecom_weight_grams"] = req.EcomWeightGrams.Value
		}
	}
	if req.EcomMinOrder != nil {
		if *req.EcomMinOrder < 1 {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Min order minimal 1"}
		}
		updates["ecom_min_order"] = *req.EcomMinOrder
	}
	if req.EcomDescription != nil {
		if req.EcomDescription.Null {
			updates["ecom_description"] = nil
		} else {
			updates["ecom_description"] = req.EcomDescription.Value
		}
	}
	if req.EcomImage != nil {
		if req.EcomImage.Null {
			updates["ecom_image"] = nil
		} else {
			updates["ecom_image"] = req.EcomImage.Value
		}
	}
	if req.EcomImages != nil {
		// Marshal ke JSON string — GORM datatypes.JSON accept raw bytes.
		imgs := *req.EcomImages
		if len(imgs) == 0 {
			updates["ecom_images"] = nil
		} else {
			b, _ := json.Marshal(imgs)
			updates["ecom_images"] = string(b)
		}
	}
	if req.EcomCategoryID != nil {
		if req.EcomCategoryID.Null || req.EcomCategoryID.Value == "" {
			updates["ecom_category_id"] = nil
		} else {
			// Validate exists — cegah FK violation dari FE bug.
			var count int64
			s.DB.Model(&entity.EcomCategory{}).Where("id = ? AND deleted_at IS NULL", req.EcomCategoryID.Value).Count(&count)
			if count == 0 {
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Kategori ecom tidak ditemukan"}
			}
			updates["ecom_category_id"] = req.EcomCategoryID.Value
		}
	}

	if len(updates) == 0 {
		// No changes — return current state.
		resp := toEcomAdminProductResponse(&product)
		return &resp, nil
	}

	updates["updated_at"] = time.Now()

	if err := s.DB.Model(&entity.Product{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		s.Log.Error().Err(err).Str("product_id", id).Msg("Failed to update ecom fields")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save"}
	}

	// Log audit — best-effort, tidak block success response.
	s.Log.Info().
		Str("product_id", id).
		Str("changed_by", changedBy).
		Interface("changes", updates).
		Msg("ecom fields updated")

	// Re-fetch to return latest state.
	var updated entity.Product
	if err := s.DB.Preload("EcomCategory").Where("id = ?", id).First(&updated).Error; err != nil {
		// Fallback ke pre-update copy — rare edge case.
		resp := toEcomAdminProductResponse(&product)
		return &resp, nil
	}
	resp := toEcomAdminProductResponse(&updated)
	return &resp, nil
}

func toEcomAdminProductResponse(p *entity.Product) dto.EcomAdminProductResponse {
	catName := ""
	if p.EcomCategory != nil {
		if p.EcomCategory.NameID != "" {
			catName = p.EcomCategory.NameID
		} else {
			catName = p.EcomCategory.Name
		}
	}
	return dto.EcomAdminProductResponse{
		ID:                p.ID,
		Name:              p.Name,
		NameID:            p.NameID,
		SKU:               p.SKU,
		StockPOS:          p.Stock,
		SellingPrice:      p.SellingPrice,
		MemberPrice:       p.MemberPrice,
		StockEcom:         p.StockEcom,
		EcomPrice:         p.EcomPrice,
		EcomMemberPrice:   p.EcomMemberPrice,
		EcomIsAvailable:   p.EcomIsAvailable,
		EcomDescription:   p.EcomDescription,
		EcomImage:         p.EcomImage,
		EcomImages:        decodeEcomImages(p.EcomImages),
		EcomCategoryID:    p.EcomCategoryID,
		EcomCategoryName:  catName,
		EcomWeightGrams:   p.EcomWeightGrams,
		EcomMinOrder:      p.EcomMinOrder,
		Image:             p.Image,
	}
}

// decodeEcomImages — parse JSON column ke []string. NULL/kosong = empty slice
// (bukan nil) supaya FE render `[].map` tanpa null guard.
func decodeEcomImages(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}
