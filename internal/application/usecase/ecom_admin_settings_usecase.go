package usecase

import (
	"context"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminSettingsService — singleton ecom settings, id='default'.
// Sprint 4 Chunk 5 (31 Jul 2026).
type EcomAdminSettingsService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAdminSettingsService(ctx context.Context, db *gorm.DB) *EcomAdminSettingsService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminSettingsService{DB: db, Log: logger}
}

// Get — return singleton row. Auto-create kalau belum ada (defensive vs
// migration slip). Public storefront juga bisa fetch subset (mis. WA number)
// tapi ini yang gate admin, return full.
func (s *EcomAdminSettingsService) Get() (*entity.EcomSettings, *dto.ApiError) {
	var settings entity.EcomSettings
	if err := s.DB.First(&settings, "id = ?", "default").Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			settings = entity.EcomSettings{ID: "default"}
			if e := s.DB.Create(&settings).Error; e != nil {
				s.Log.Error().Err(e).Msg("Failed to create default settings row")
				return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal inisialisasi settings"}
			}
		} else {
			s.Log.Error().Err(err).Msg("Failed to load settings")
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil settings"}
		}
	}
	// Sprint 5 Chunk 7 — parse JSON kolom ke virtual []string supaya FE
	// dapat array langsung (bukan raw JSON string).
	if settings.PinnedProductIDs != nil && *settings.PinnedProductIDs != "" {
		_ = json.Unmarshal([]byte(*settings.PinnedProductIDs), &settings.PinnedProductIDsParsed)
	}
	if settings.FeaturedCategoryIDs != nil && *settings.FeaturedCategoryIDs != "" {
		_ = json.Unmarshal([]byte(*settings.FeaturedCategoryIDs), &settings.FeaturedCategoryIDsParsed)
	}
	if settings.PinnedProductIDsParsed == nil {
		settings.PinnedProductIDsParsed = []string{}
	}
	if settings.FeaturedCategoryIDsParsed == nil {
		settings.FeaturedCategoryIDsParsed = []string{}
	}
	return &settings, nil
}

// Update — merge patch dari request. Field NULL/omit di request tidak
// diubah (partial update). Pattern map supaya tidak overwrite dengan zero
// value yang tidak disengaja.
func (s *EcomAdminSettingsService) Update(patch map[string]interface{}) (*entity.EcomSettings, *dto.ApiError) {
	// Guard: hanya field yang di-whitelist boleh di-set.
	allowed := map[string]bool{
		"min_order_amount":            true,
		"wa_contact_number":           true,
		"wa_pretext":                  true,
		"announcement_bar_enabled":    true,
		"announcement_bar_text":       true,
		"announcement_bar_cta_label":  true,
		"announcement_bar_cta_url":    true,
		"store_name":                  true,
		"store_email":                 true,
		"store_pickup_address":        true,
		"store_pickup_phone":          true,
		"store_pickup_area_id":        true,
		"payment_pg_enabled":          true,
		"payment_manual_enabled":      true,
		"notif_order_email_enabled":   true,
		// Sprint 5 Chunk 7 — Homepage CMS
		"hero_kicker":            true,
		"hero_title":             true,
		"hero_subtitle":          true,
		"hero_cta_label":         true,
		"hero_cta_url":           true,
		"pinned_product_ids":     true,
		"featured_category_ids":  true,
	}
	clean := map[string]interface{}{}
	for k, v := range patch {
		if !allowed[k] {
			continue
		}
		// JSON columns — marshal []interface{} ke JSON string supaya MySQL
		// tidak reject typed slice. Kalau v sudah string, biarkan pass-through.
		if k == "pinned_product_ids" || k == "featured_category_ids" {
			switch tv := v.(type) {
			case nil:
				clean[k] = nil
			case string:
				clean[k] = tv
			default:
				b, err := json.Marshal(tv)
				if err != nil {
					continue
				}
				clean[k] = string(b)
			}
			continue
		}
		clean[k] = v
	}
	if len(clean) == 0 {
		return s.Get() // no-op
	}
	// Ensure singleton row exists
	if _, fail := s.Get(); fail != nil {
		return nil, fail
	}
	if err := s.DB.Model(&entity.EcomSettings{}).
		Where("id = ?", "default").
		Updates(clean).Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to update settings")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan settings"}
	}
	return s.Get()
}
