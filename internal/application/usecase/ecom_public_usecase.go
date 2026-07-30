package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomPublicService — public storefront queries. Semua filter WHERE
// ecom_is_available=1 AND stock_ecom>0 AND deleted_at IS NULL — customer
// hanya lihat produk yang benar-benar ready to buy.
type EcomPublicService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomPublicService(ctx context.Context, db *gorm.DB) *EcomPublicService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomPublicService{DB: db, Log: logger}
}

// ListCategories — kategori ecom (bukan POS) yang aktif + punya minimal 1
// produk siap tayang. Migration 000053 pisahkan taxonomy ecom dari POS.
func (s *EcomPublicService) ListCategories() ([]dto.EcomCategoryResponse, *dto.ApiError) {
	rows := []struct {
		ID           string
		Name         string
		NameID       string
		IconName    string
		SortOrder    int
		IsActive     bool
		ProductCount int
	}{}

	err := s.DB.Table("ecom_categories c").
		Select(`c.id, c.name, c.name_id, c.icon_name, c.sort_order, c.is_active,
			COALESCE((SELECT COUNT(*) FROM products p
				WHERE p.ecom_category_id = c.id
					AND p.deleted_at IS NULL
					AND p.is_active = 1
					AND p.ecom_is_available = 1
					AND p.stock_ecom > 0), 0) AS product_count`).
		Where("c.deleted_at IS NULL AND c.is_active = 1").
		Having("product_count > 0").
		Order("c.sort_order ASC, c.name ASC").
		Scan(&rows).Error
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to list ecom categories")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch categories"}
	}

	items := make([]dto.EcomCategoryResponse, 0, len(rows))
	for _, r := range rows {
		items = append(items, dto.EcomCategoryResponse{
			ID:           r.ID,
			Name:         r.Name,
			NameID:       r.NameID,
			IconName:    r.IconName,
			SortOrder:    r.SortOrder,
			IsActive:     r.IsActive,
			ProductCount: r.ProductCount,
		})
	}
	return items, nil
}

// ListProducts — cursor pagination + filter kategori + search + sort.
// Semua WHERE clause enforce ecom_is_available + stock_ecom > 0.
// ListProductsFilter — Sprint 2 #7 — filter tambahan untuk Cari/Katalog page.
type ListProductsFilter struct {
	MinPrice float64
	MaxPrice float64
	InStock  bool // include stock <= 5 as low stock? Handled at FE for badge
}

func (s *EcomPublicService) ListProducts(categoryID, search, sort, cursor string, limit int) (*dto.EcomProductListResponse, *dto.ApiError) {
	return s.ListProductsFiltered(categoryID, search, sort, cursor, limit, ListProductsFilter{})
}

func (s *EcomPublicService) ListProductsFiltered(categoryID, search, sort, cursor string, limit int, filter ListProductsFilter) (*dto.EcomProductListResponse, *dto.ApiError) {
	var products []entity.Product

	q := s.DB.Model(&entity.Product{}).
		Where("deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1 AND stock_ecom > 0")

	if categoryID != "" {
		q = q.Where("ecom_category_id = ?", categoryID)
	}
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("name LIKE ? OR name_id LIKE ? OR sku LIKE ?", like, like, like)
	}

	// Filter harga — pakai COALESCE(ecom_price, selling_price) supaya konsisten
	// dengan sort. MinPrice/MaxPrice 0 = tidak diapply.
	if filter.MinPrice > 0 {
		q = q.Where("COALESCE(ecom_price, selling_price) >= ?", filter.MinPrice)
	}
	if filter.MaxPrice > 0 {
		q = q.Where("COALESCE(ecom_price, selling_price) <= ?", filter.MaxPrice)
	}

	// Sort options: newest (default), price_asc, price_desc, name.
	orderClause := ""
	switch sort {
	case "price_asc":
		orderClause = "COALESCE(ecom_price, selling_price) ASC"
	case "price_desc":
		orderClause = "COALESCE(ecom_price, selling_price) DESC"
	case "name":
		orderClause = "name_id ASC"
	default:
		orderClause = "created_at DESC"
	}

	// Cursor pagination hanya work untuk default sort (created_at DESC).
	// Untuk sort lain, pakai offset-less pagination via id > cursor. MVP: skip.
	if cursor != "" && sort == "" {
		if t, err := time.Parse(time.RFC3339, cursor); err == nil {
			q = q.Where("created_at < ?", t)
		}
	}

	if err := q.Preload("EcomCategory").Preload("Category").
		Order(orderClause).
		Limit(limit).
		Find(&products).Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to list ecom products")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch products"}
	}

	items := make([]dto.EcomProductListItem, 0, len(products))
	for i := range products {
		items = append(items, toEcomListItem(&products[i]))
	}

	nextCursor := ""
	if len(products) == limit && limit > 0 && sort == "" {
		nextCursor = products[len(products)-1].CreatedAt.Format(time.RFC3339)
	}

	return &dto.EcomProductListResponse{
		Items:      items,
		NextCursor: nextCursor,
	}, nil
}

// GetProduct — detail full untuk PDP. Include description + tiers.
// Sprint 3 #16 (30 Jul 2026): habis (stock_ecom=0) TETAP boleh di-view supaya
// customer bisa subscribe restock alert. ListProducts (katalog) tetap filter
// stock_ecom > 0 supaya habis tidak muncul di browse.
func (s *EcomPublicService) GetProduct(id string) (*dto.EcomProductDetail, *dto.ApiError) {
	var product entity.Product
	err := s.DB.Preload("EcomCategory").Preload("Category").
		Where("id = ? AND deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1", id).
		First(&product).Error
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak tersedia"}
	}

	base := toEcomListItem(&product)
	detail := &dto.EcomProductDetail{EcomProductListItem: base}
	if product.EcomDescription != nil {
		detail.Description = *product.EcomDescription
	}
	// Gallery images. Decode JSON dari ecom_images. Kalau kosong, fallback
	// ke [Image] tunggal supaya FE swiper tetap render 1 slide. Kalau Image
	// juga kosong, kirim empty slice — FE tampilkan placeholder Package icon.
	if imgs := decodeEcomImages(product.EcomImages); len(imgs) > 0 {
		detail.Images = imgs
	} else if base.Image != "" {
		detail.Images = []string{base.Image}
	} else {
		detail.Images = []string{}
	}

	// Load tier grosir (target 'all_customers' saja untuk public storefront).
	var tiers []entity.ProductPriceTier
	tierErr := s.DB.Where("product_id = ? AND deleted_at IS NULL AND target_type = 'all_customers'", id).
		Order("min_qty ASC").Find(&tiers).Error
	if tierErr == nil {
		for _, t := range tiers {
			// Skip tier yang sudah expire.
			if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
				continue
			}
			detail.Tiers = append(detail.Tiers, dto.EcomProductTierPrice{
				MinQty: t.MinQty,
				Price:  t.Price,
				Note:   t.Note,
			})
		}
	}

	return detail, nil
}

// GetRelated — produk sejenis untuk PDP cross-sell. Prefer same ecom_category
// (kalau ada), fallback ke category POS. Exclude current product + out-of-stock.
// Sort by created_at DESC (proxy "produk baru dulu"). Limit default 6.
func (s *EcomPublicService) GetRelated(productID string, limit int) ([]dto.EcomProductListItem, *dto.ApiError) {
	if limit <= 0 || limit > 20 {
		limit = 6
	}
	// Ambil current product untuk tahu category-nya.
	var current entity.Product
	if err := s.DB.Select("id, category_id, ecom_category_id").
		Where("id = ? AND deleted_at IS NULL", productID).First(&current).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak ditemukan"}
	}

	// Query produk lain di category sama.
	q := s.DB.Model(&entity.Product{}).
		Where("id <> ? AND deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1 AND stock_ecom > 0", productID)
	// Prefer ecom_category kalau ada, fallback ke category POS.
	if current.EcomCategoryID != nil && *current.EcomCategoryID != "" {
		q = q.Where("ecom_category_id = ?", *current.EcomCategoryID)
	} else if current.CategoryID != "" {
		q = q.Where("category_id = ?", current.CategoryID)
	}

	var products []entity.Product
	if err := q.Preload("EcomCategory").Preload("Category").
		Order("created_at DESC").Limit(limit).Find(&products).Error; err != nil {
		s.Log.Error().Err(err).Msg("GetRelated query failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil produk sejenis"}
	}

	out := make([]dto.EcomProductListItem, 0, len(products))
	for _, p := range products {
		out = append(out, toEcomListItem(&p))
	}
	return out, nil
}

func toEcomListItem(p *entity.Product) dto.EcomProductListItem {
	price := p.SellingPrice
	if p.EcomPrice != nil {
		price = *p.EcomPrice
	}
	var memberPrice *float64
	if p.EcomMemberPrice != nil {
		memberPrice = p.EcomMemberPrice
	} else if p.MemberPrice != nil {
		memberPrice = p.MemberPrice
	}
	// Ecom category dulu; fallback ke POS Category kalau produk belum di-tag
	// ecom_category (mis. produk lama sebelum migration).
	catName := ""
	catID := ""
	if p.EcomCategory != nil {
		catID = p.EcomCategory.ID
		catName = p.EcomCategory.NameID
		if catName == "" {
			catName = p.EcomCategory.Name
		}
	} else if p.Category != nil {
		catID = p.Category.ID
		catName = p.Category.NameID
		if catName == "" {
			catName = p.Category.Name
		}
	}
	// ecom_image override kalau di-set, else fallback ke image POS.
	image := p.Image
	if p.EcomImage != nil && *p.EcomImage != "" {
		image = *p.EcomImage
	}
	return dto.EcomProductListItem{
		ID:           p.ID,
		Name:         p.Name,
		NameID:       p.NameID,
		SKU:          p.SKU,
		CategoryID:   catID,
		CategoryName: catName,
		Image:        image,
		Price:        price,
		MemberPrice:  memberPrice,
		Stock:        p.StockEcom,
		WeightGrams:  p.EcomWeightGrams,
		MinOrder:     p.EcomMinOrder,
		IsLowStock:   p.StockEcom <= 5,
	}
}
