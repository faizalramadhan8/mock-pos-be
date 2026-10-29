package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomPublicService — public storefront queries. Filter dasarnya
// ecom_is_available=1 AND stock>0; `location` opsional — kosong berarti
// SEMUA toko (beranda lintas toko), diisi berarti satu toko saja. Customer
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
// ListCategories — `location` kosong = hitung produk dari SEMUA toko
// (beranda lintas toko); diisi = hanya toko itu (halaman toko).
func (s *EcomPublicService) ListCategories(location string) ([]dto.EcomCategoryResponse, *dto.ApiError) {
	// "IN ?" bukan "IN (?)": GORM menulis slice lengkap dengan tanda kurungnya
	// sendiri, jadi menambahkan kurung di sini menghasilkan "IN (('home'))" —
	// MySQL membacanya sebagai row constructor dan menolaknya.
	locFilter := " AND p.location IN ?"
	locArgs := []interface{}{TempatTayang(s.DB)}
	if l := strings.TrimSpace(location); l != "" {
		loc := TempatSah(l)
		if !TempatBolehTayang(s.DB, loc) {
			return []dto.EcomCategoryResponse{}, nil
		}
		locFilter = " AND p.location = ?"
		locArgs = []interface{}{loc}
	}
	rows := []struct {
		ID           string
		Name         string
		NameID       string
		IconName     string
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
					AND p.stock > 0`+locFilter+`), 0) AS product_count`, locArgs...).
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
			IconName:     r.IconName,
			SortOrder:    r.SortOrder,
			IsActive:     r.IsActive,
			ProductCount: r.ProductCount,
		})
	}
	return items, nil
}

// ListProducts — cursor pagination + filter kategori + search + sort.
// Semua WHERE clause enforce ecom_is_available + stock > 0.
// `stock` = stok di rumah; `stock` adalah rak Bekasi dan tidak dijual online.
// ListProductsFilter — Sprint 2 #7 — filter tambahan untuk Cari/Katalog page.
type ListProductsFilter struct {
	MinPrice float64
	MaxPrice float64
	InStock  bool // include stock <= 5 as low stock? Handled at FE for badge
	// Location — "" berarti SEMUA toko (beranda lintas toko), "home"/"store"
	// membatasi ke satu toko (halaman toko).
	Location string
}

func (s *EcomPublicService) ListProducts(categoryID, search, sort, cursor string, limit int) (*dto.EcomProductListResponse, *dto.ApiError) {
	return s.ListProductsFiltered(categoryID, search, sort, cursor, limit, ListProductsFilter{})
}

func (s *EcomPublicService) ListProductsFiltered(categoryID, search, sort, cursor string, limit int, filter ListProductsFilter) (*dto.EcomProductListResponse, *dto.ApiError) {
	var products []entity.Product

	q := s.DB.Model(&entity.Product{}).
		Where("deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1 AND stock > 0")

	// Tanpa `location` = semua toko yang SEDANG TAYANG. Penyaring
	// `location = 'home'` yang dulu dipasang mati di sini adalah yang membuat
	// storefront cuma bisa melayani satu toko; penggantinya bukan "tanpa
	// penyaring", melainkan daftar toko yang saklarnya menyala.
	if l := strings.TrimSpace(filter.Location); l != "" {
		loc := TempatSah(l)
		if !TempatBolehTayang(s.DB, loc) {
			return &dto.EcomProductListResponse{Items: []dto.EcomProductListItem{}}, nil
		}
		q = q.Where("location = ?", loc)
	} else {
		q = q.Where("location IN ?", TempatTayang(s.DB))
	}

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
		it := toEcomListItem(&products[i])
		s.isiNamaToko(&it)
		items = append(items, it)
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
// Sprint 3 #16 (30 Jul 2026): habis (stock=0) TETAP boleh di-view supaya
// customer bisa subscribe restock alert. ListProducts (katalog) tetap filter
// stock > 0 supaya habis tidak muncul di browse.
func (s *EcomPublicService) GetProduct(id string) (*dto.EcomProductDetail, *dto.ApiError) {
	var product entity.Product
	err := s.DB.Preload("EcomCategory").Preload("Category").
		Where("id = ? AND deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1", id).
		First(&product).Error
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak tersedia"}
	}
	// Toko yang belum tayang: produknya memang tidak ada dari sudut pandang
	// storefront. Tanpa ini, tautan langsung ke id produk Bekasi tetap bisa
	// dibuka sebelum tokonya siap.
	if !TempatBolehTayang(s.DB, product.Location) {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak tersedia"}
	}

	base := toEcomListItem(&product)
	s.isiNamaToko(&base)
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
		now := time.Now()
		for i := range tiers {
			t := &tiers[i]
			// Tier di luar jendela jadwalnya tidak dikirim sama sekali. Itu
			// sebabnya promo terjadwal tidak perlu disaring lagi di sisi
			// storefront: yang belum mulai memang tidak pernah sampai ke sana.
			if !tierSedangBerlaku(t, now) {
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
		// Sengaja dibatasi ke toko yang SAMA: barang "sejenis" dari toko lain
		// berarti paket kedua dengan ongkir sendiri — itu bukan rekomendasi
		// yang menolong, itu menambah biaya tanpa diminta.
		Where("id <> ? AND location = ? AND deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1 AND stock > 0", productID, TempatSah(current.Location))
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
		it := toEcomListItem(&p)
		s.isiNamaToko(&it)
		out = append(out, it)
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
		Stock:        p.Stock,
		WeightGrams:  p.EcomWeightGrams,
		MinOrder:     p.EcomMinOrder,
		IsLowStock:   p.Stock <= 5,
		Location:     TempatSah(p.Location),
	}
}

// isiNamaToko — lengkapi nama toko di kartu produk. Dipisah dari
// toEcomListItem karena fungsi itu murni (tanpa DB); nama tokonya di-cache
// 60 detik, jadi memanggilnya per kartu tidak menimbulkan query beruntun.
func (s *EcomPublicService) isiNamaToko(it *dto.EcomProductListItem) {
	it.StoreName = NamaToko(s.DB, it.Location)
}
