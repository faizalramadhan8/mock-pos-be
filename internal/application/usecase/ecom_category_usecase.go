package usecase

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomCategoryService — CRUD kategori khusus storefront ecom. Terpisah dari
// POS Category (migration 000053). Public list dipakai storefront browse,
// admin CRUD dipakai admin panel ecom.
type EcomCategoryService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomCategoryService(ctx context.Context, db *gorm.DB) *EcomCategoryService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomCategoryService{DB: db, Log: logger}
}

// List — untuk admin (include inactive) atau public (activeOnly=true, filter
// stok > 0 via product_count).
func (s *EcomCategoryService) List(activeOnly bool) ([]dto.EcomCategoryResponse, *dto.ApiError) {
	var cats []entity.EcomCategory
	q := s.DB.Model(&entity.EcomCategory{}).Where("deleted_at IS NULL")
	if activeOnly {
		q = q.Where("is_active = 1")
	}
	if err := q.Order("sort_order ASC, name ASC").Find(&cats).Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to list ecom categories")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch"}
	}

	// Count produk per kategori — 1 query grouped supaya scalable.
	// Filter cuma yang eligible tampil di storefront (kalau public).
	type countRow struct {
		EcomCategoryID string
		Cnt            int
	}
	var counts []countRow
	countQ := s.DB.Model(&entity.Product{}).
		Select("ecom_category_id, COUNT(*) as cnt").
		Where("deleted_at IS NULL AND ecom_category_id IS NOT NULL")
	if activeOnly {
		countQ = countQ.Where("is_active = 1 AND ecom_is_available = 1 AND stock_ecom > 0")
	}
	countQ.Group("ecom_category_id").Scan(&counts)
	countMap := map[string]int{}
	for _, c := range counts {
		countMap[c.EcomCategoryID] = c.Cnt
	}

	items := make([]dto.EcomCategoryResponse, 0, len(cats))
	for _, c := range cats {
		items = append(items, dto.EcomCategoryResponse{
			ID:           c.ID,
			Name:         c.Name,
			NameID:       c.NameID,
			IconName:    c.IconName,
			SortOrder:    c.SortOrder,
			IsActive:     c.IsActive,
			ProductCount: countMap[c.ID],
		})
	}
	return items, nil
}

func (s *EcomCategoryService) Create(req dto.EcomCategoryRequest) (*dto.EcomCategoryResponse, *dto.ApiError) {
	if req.Name == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Nama kategori wajib"}
	}
	c := entity.EcomCategory{
		ID:        uuid.New().String(),
		Name:      req.Name,
		NameID:    req.NameID,
		IconName: req.IconName,
		SortOrder: req.SortOrder,
		IsActive:  true,
	}
	if err := s.DB.Create(&c).Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to create ecom category")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan"}
	}
	resp := dto.EcomCategoryResponse{
		ID: c.ID, Name: c.Name, NameID: c.NameID, IconName: c.IconName,
		SortOrder: c.SortOrder, IsActive: c.IsActive,
	}
	return &resp, nil
}

func (s *EcomCategoryService) Update(id string, req dto.EcomCategoryRequest) (*dto.EcomCategoryResponse, *dto.ApiError) {
	var c entity.EcomCategory
	if err := s.DB.Where("id = ? AND deleted_at IS NULL", id).First(&c).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Kategori tidak ditemukan"}
	}
	if req.Name != "" {
		c.Name = req.Name
	}
	c.NameID = req.NameID
	c.IconName = req.IconName
	c.SortOrder = req.SortOrder
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}
	if err := s.DB.Save(&c).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan"}
	}
	resp := dto.EcomCategoryResponse{
		ID: c.ID, Name: c.Name, NameID: c.NameID, IconName: c.IconName,
		SortOrder: c.SortOrder, IsActive: c.IsActive,
	}
	return &resp, nil
}

// Delete — soft delete. Products.ecom_category_id di-set NULL by FK CASCADE.
func (s *EcomCategoryService) Delete(id string) *dto.ApiError {
	if err := s.DB.Where("id = ?", id).Delete(&entity.EcomCategory{}).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal hapus"}
	}
	return nil
}
