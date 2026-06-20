package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/rs/zerolog"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ProductService struct {
	Log             *zerolog.Logger
	DB              *gorm.DB
	Repo            *repository.ProductRepository
	HistoryRepo     *repository.ProductPriceHistoryRepository
	TierRepo        *repository.ProductPriceTierRepository
	TierHistoryRepo *repository.ProductPriceTierHistoryRepository
	MemberRepo      *repository.MemberRepository
}

func NewProductService(ctx context.Context, db *gorm.DB) *ProductService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &ProductService{
		Log:             logger,
		DB:              db,
		Repo:            repository.NewProductRepository(ctx, db),
		HistoryRepo:     repository.NewProductPriceHistoryRepository(ctx, db),
		TierRepo:        repository.NewProductPriceTierRepository(ctx, db),
		TierHistoryRepo: repository.NewProductPriceTierHistoryRepository(ctx, db),
		MemberRepo:      repository.NewMemberRepository(ctx, db),
	}
}

// logPriceChange closes any active row of the given (product, type) and
// inserts a fresh active row at `now`. Errors are logged but never block the
// caller — price-history is an audit trail, not a transactional dependency.
func (s *ProductService) logPriceChange(productID, priceType string, price float64, changedBy *string, note string) {
	now := time.Now()
	if err := s.HistoryRepo.CloseActive(productID, priceType, now); err != nil {
		s.Log.Warn().Err(err).Str("product_id", productID).Str("type", priceType).Msg("price history: close active failed")
	}
	row := &entity.ProductPriceHistory{
		ID:        uuid.New().String(),
		ProductID: productID,
		PriceType: priceType,
		Price:     price,
		Status:    "active",
		StartDate: now,
		ChangedBy: changedBy,
		Note:      note,
	}
	if err := s.HistoryRepo.Create(row); err != nil {
		s.Log.Warn().Err(err).Str("product_id", productID).Str("type", priceType).Msg("price history: insert failed")
	}
}

func (s *ProductService) GetAll(search, categoryID, supplierID string, page, limit int) ([]dto.ProductResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	products, total, err := s.Repo.FindAll(search, categoryID, supplierID, limit, offset)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch products")
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch products"}
	}

	var result []dto.ProductResponse
	for _, p := range products {
		result = append(result, s.toResponse(&p))
	}
	return result, total, nil
}

func (s *ProductService) GetByID(id string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) GetBySKU(sku string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindBySKU(sku)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) GetLowStock() ([]dto.ProductResponse, *dto.ApiError) {
	products, err := s.Repo.FindLowStock()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch low stock products")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch low stock products"}
	}

	var result []dto.ProductResponse
	for _, p := range products {
		result = append(result, s.toResponse(&p))
	}
	return result, nil
}

func (s *ProductService) Create(req dto.CreateProductRequest, userID string) (*dto.ProductResponse, *dto.ApiError) {
	exists, err := s.Repo.ExistsBySKU(req.SKU)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to check SKU"}
	}
	if exists {
		return nil, &dto.ApiError{StatusCode: fiber.ErrConflict, Message: "SKU already exists"}
	}

	product := &entity.Product{
		ID:            uuid.New().String(),
		SKU:           req.SKU,
		Barcode:       req.Barcode,
		Name:          req.Name,
		NameID:        req.NameID,
		CategoryID:    req.CategoryID,
		SupplierID:    req.SupplierID,
		PurchasePrice: req.PurchasePrice,
		SellingPrice:  req.SellingPrice,
		MemberPrice:   req.MemberPrice,
		QtyPerBox:     req.QtyPerBox,
		Stock:         req.Stock,
		Unit:          req.Unit,
		Image:         req.Image,
		MinStock:      req.MinStock,
		IsActive:      true,
	}

	if product.QtyPerBox == 0 {
		product.QtyPerBox = 1
	}

	// Wrap product create + initial stock movement dalam transaksi atomic.
	// Kalau salah satu gagal, rollback supaya tidak ada produk tanpa audit
	// trail atau audit trail tanpa produk.
	tx := s.DB.Begin()

	if err := tx.Create(product).Error; err != nil {
		tx.Rollback()
		s.Log.Error().Err(err).Msg("Failed to create product")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create product"}
	}

	// Kalau ada stok awal > 0, insert stock_movement reason='initial' supaya
	// audit trail per produk komplit. Sebelum fix ini, init stock di-set
	// langsung ke products.stock tanpa movement record → audit selisih jadi
	// false signal (total_in = 0 walau stok awal ada).
	if product.Stock > 0 {
		movement := &entity.StockMovement{
			ID:        uuid.New().String(),
			ProductID: product.ID,
			Type:      "in",
			Quantity:  product.Stock,
			UnitType:  "individual",
			UnitPrice: product.PurchasePrice,
			Reason:    "initial",
			Note:      "Stok awal saat produk dibuat",
			CreatedBy: userID,
		}
		if err := tx.Create(movement).Error; err != nil {
			tx.Rollback()
			s.Log.Error().Err(err).Msg("Failed to create initial stock movement")
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create initial stock movement"}
		}
	}

	if err := tx.Commit().Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to commit product create")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create product"}
	}

	// Seed price history with the initial regular/purchase/member rows so
	// subsequent edits already have a closed predecessor to compare against.
	// Best-effort: di luar transaksi karena history audit only (not critical).
	var changer *string
	if userID != "" {
		changer = &userID
	}
	s.logPriceChange(product.ID, "regular", product.SellingPrice, changer, "initial")
	s.logPriceChange(product.ID, "purchase", product.PurchasePrice, changer, "initial")
	if product.MemberPrice != nil && *product.MemberPrice > 0 {
		s.logPriceChange(product.ID, "member", *product.MemberPrice, changer, "initial")
	}

	p, _ := s.Repo.FindByID(product.ID)
	if p != nil {
		product = p
	}
	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) Update(id string, req dto.UpdateProductRequest, userID string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}

	// Snapshot pre-update prices so we know what actually changed.
	prevPurchase := product.PurchasePrice
	prevSelling := product.SellingPrice
	var prevMember *float64
	if product.MemberPrice != nil {
		v := *product.MemberPrice
		prevMember = &v
	}

	if req.Name != "" {
		product.Name = req.Name
	}
	if req.NameID != "" {
		product.NameID = req.NameID
	}
	if req.CategoryID != "" {
		product.CategoryID = req.CategoryID
	}
	// SupplierID: pointer — explicit null clears it, value sets it
	if req.SupplierID != nil {
		if *req.SupplierID == "" {
			product.SupplierID = nil
		} else {
			product.SupplierID = req.SupplierID
		}
	}
	if req.PurchasePrice > 0 {
		product.PurchasePrice = req.PurchasePrice
	}
	if req.SellingPrice > 0 {
		product.SellingPrice = req.SellingPrice
	}
	// MemberPrice: pointer — explicit null clears it, value sets it
	if req.MemberPrice != nil {
		if *req.MemberPrice <= 0 {
			product.MemberPrice = nil
		} else {
			product.MemberPrice = req.MemberPrice
		}
	}
	if req.QtyPerBox > 0 {
		product.QtyPerBox = req.QtyPerBox
	}
	if req.Unit != "" {
		product.Unit = req.Unit
	}
	if req.Image != "" {
		product.Image = req.Image
	}
	if req.MinStock >= 0 {
		product.MinStock = req.MinStock
	}
	if req.SKU != "" {
		product.SKU = req.SKU
	}
	if req.Barcode != "" {
		product.Barcode = req.Barcode
	}
	if req.Stock != nil {
		product.Stock = *req.Stock
	}

	if err := s.Repo.Update(product); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update product")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update product"}
	}

	// Log price changes (only when value actually moved). Audit trail
	// is best-effort: failures are logged inside logPriceChange, never block.
	var changer *string
	if userID != "" {
		changer = &userID
	}
	if product.PurchasePrice != prevPurchase {
		s.logPriceChange(product.ID, "purchase", product.PurchasePrice, changer, "")
	}
	if product.SellingPrice != prevSelling {
		s.logPriceChange(product.ID, "regular", product.SellingPrice, changer, "")
	}
	memberChanged := false
	switch {
	case prevMember == nil && product.MemberPrice != nil:
		memberChanged = true
	case prevMember != nil && product.MemberPrice == nil:
		memberChanged = true
	case prevMember != nil && product.MemberPrice != nil && *prevMember != *product.MemberPrice:
		memberChanged = true
	}
	if memberChanged {
		var newMember float64
		if product.MemberPrice != nil {
			newMember = *product.MemberPrice
		}
		s.logPriceChange(product.ID, "member", newMember, changer, "")
	}

	resp := s.toResponse(product)
	return &resp, nil
}

func (s *ProductService) AdjustStock(id string, delta int) (*dto.ProductResponse, *dto.ApiError) {
	if err := s.Repo.AdjustStock(id, delta); err != nil {
		s.Log.Error().Err(err).Msg("Failed to adjust stock")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to adjust stock"}
	}

	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	resp := s.toResponse(product)
	return &resp, nil
}

// Delete soft-deletes a product. Previous orders that reference it keep their
// own name/price snapshots in order_items, so history stays intact.
func (s *ProductService) Delete(id string) *dto.ApiError {
	if _, err := s.Repo.FindByID(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete product")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete product"}
	}
	return nil
}

func (s *ProductService) ToggleActive(id string) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}

	product.IsActive = !product.IsActive
	if err := s.Repo.Update(product); err != nil {
		s.Log.Error().Err(err).Msg("Failed to toggle product active status")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update product"}
	}

	resp := s.toResponse(product)
	return &resp, nil
}

// SetRedeemable explicitly sets the is_redeemable flag. Used by the
// "Katalog Tebus Poin" page where admin add/remove products from the
// catalog. Body uses explicit value (not toggle) so the action is
// idempotent — re-clicking "Tambah" tidak flip ke off.
func (s *ProductService) SetRedeemable(id string, redeemable bool) (*dto.ProductResponse, *dto.ApiError) {
	product, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	product.IsRedeemable = redeemable
	if err := s.Repo.Update(product); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update is_redeemable")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update product"}
	}
	resp := s.toResponse(product)
	return &resp, nil
}

// GetPriceHistory returns chronological price changes for a product. Optional
// priceType filter — empty = all (regular + member + purchase).
func (s *ProductService) GetPriceHistory(productID, priceType string) ([]dto.ProductPriceHistoryResponse, *dto.ApiError) {
	if _, err := s.Repo.FindByID(productID); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	rows, err := s.HistoryRepo.FindByProduct(productID, priceType)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch price history")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch price history"}
	}
	out := make([]dto.ProductPriceHistoryResponse, 0, len(rows))
	for _, r := range rows {
		row := dto.ProductPriceHistoryResponse{
			ID:        r.ID,
			ProductID: r.ProductID,
			PriceType: r.PriceType,
			Price:     r.Price,
			Status:    r.Status,
			StartDate: r.StartDate.Format(time.RFC3339),
			ChangedBy: r.ChangedBy,
			Note:      r.Note,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		}
		if r.EndDate != nil {
			s := r.EndDate.Format(time.RFC3339)
			row.EndDate = &s
		}
		out = append(out, row)
	}
	return out, nil
}

// NextSKU — generate SKU auto untuk produk baru dengan prefix tertentu
// (biasanya dari nama kategori). Cek termasuk soft-deleted rows supaya tidak
// collide dengan SKU yang "stuck" di soft-delete. Format: <PREFIX>-<NNN>
// dengan zero-pad 3 digit.
func (s *ProductService) NextSKU(prefix string) (string, *dto.ApiError) {
	if prefix == "" {
		prefix = "GEN"
	}
	maxNum, err := s.Repo.FindMaxSKUNumberByPrefix(prefix)
	if err != nil {
		return "", &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	next := maxNum + 1
	if next < 1 {
		next = 1
	}
	return fmt.Sprintf("%s-%03d", prefix, next), nil
}

func (s *ProductService) toResponse(p *entity.Product) dto.ProductResponse {
	resp := dto.ProductResponse{
		ID:            p.ID,
		SKU:           p.SKU,
		Barcode:       p.Barcode,
		Name:          p.Name,
		NameID:        p.NameID,
		CategoryID:    p.CategoryID,
		SupplierID:    p.SupplierID,
		PurchasePrice: p.PurchasePrice,
		SellingPrice:  p.SellingPrice,
		MemberPrice:   p.MemberPrice,
		QtyPerBox:     p.QtyPerBox,
		Stock:         p.Stock,
		Unit:          p.Unit,
		Image:         p.Image,
		MinStock:      p.MinStock,
		IsActive:      p.IsActive,
		IsRedeemable:  p.IsRedeemable,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
	}
	if p.Category != nil {
		cat := dto.CategoryResponse{
			ID:     p.Category.ID,
			Name:   p.Category.Name,
			NameID: p.Category.NameID,
			Icon:   p.Category.Icon,
			Color:  p.Category.Color,
		}
		resp.Category = &cat
	}
	if p.Supplier != nil {
		sup := dto.SupplierResponse{
			ID:        p.Supplier.ID,
			Name:      p.Supplier.Name,
			Phone:     p.Supplier.Phone,
			Email:     p.Supplier.Email,
			Address:   p.Supplier.Address,
			CreatedAt: p.Supplier.CreatedAt.Format(time.RFC3339),
		}
		resp.Supplier = &sup
	}
	// PriceTiers: best-effort fetch — kalau gagal, biarkan kosong, tidak
	// block response. Tier-aware pricing tetap punya fallback ke selling
	// price kalau tidak ada tier.
	tiers, err := s.TierRepo.FindByProduct(p.ID)
	if err == nil && len(tiers) > 0 {
		resp.PriceTiers = make([]dto.ProductPriceTierResponse, 0, len(tiers))
		for _, t := range tiers {
			resp.PriceTiers = append(resp.PriceTiers, tierToResponse(&t))
		}
	}
	return resp
}

func tierToResponse(t *entity.ProductPriceTier) dto.ProductPriceTierResponse {
	out := dto.ProductPriceTierResponse{
		ID:         t.ID,
		ProductID:  t.ProductID,
		MinQty:     t.MinQty,
		Price:      t.Price,
		TargetType: t.TargetType,
		Note:       t.Note,
		CreatedAt:  t.CreatedAt.Format(time.RFC3339),
	}
	if len(t.Members) > 0 {
		out.Members = make([]dto.ProductPriceTierMemberRef, 0, len(t.Members))
		for _, m := range t.Members {
			out.Members = append(out.Members, dto.ProductPriceTierMemberRef{
				ID:    m.ID,
				Name:  m.Name,
				Phone: m.Phone,
			})
		}
	}
	return out
}

// ─── Price-tier CRUD ────────────────────────────────────────────────────

func (s *ProductService) ListPriceTiers(productID string) ([]dto.ProductPriceTierResponse, *dto.ApiError) {
	if _, err := s.Repo.FindByID(productID); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	tiers, err := s.TierRepo.FindByProduct(productID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch tiers"}
	}
	out := make([]dto.ProductPriceTierResponse, 0, len(tiers))
	for _, t := range tiers {
		out = append(out, tierToResponse(&t))
	}
	return out, nil
}

func (s *ProductService) buildTierFromRequest(productID string, req dto.SavePriceTierRequest) (*entity.ProductPriceTier, *dto.ApiError) {
	tier := &entity.ProductPriceTier{
		ProductID:  productID,
		MinQty:     req.MinQty,
		Price:      req.Price,
		TargetType: req.TargetType,
		Note:       req.Note,
	}
	if req.TargetType == "member_specific" {
		if len(req.MemberIDs) == 0 {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Tier 'member_specific' wajib pilih minimal 1 member"}
		}
		// Hydrate Members slice so GORM many2many insert works on Create.
		members := make([]entity.Member, 0, len(req.MemberIDs))
		for _, id := range req.MemberIDs {
			m, err := s.MemberRepo.FindByID(id)
			if err != nil {
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Member tidak ditemukan: " + id}
			}
			members = append(members, *m)
		}
		tier.Members = members
	}
	return tier, nil
}

// logTierChange writes audit row(s) ke product_price_tier_history.
// action: "create" | "update" | "delete".
// Untuk "update"/"delete", close active row dulu (status=inactive, end_date=now).
// Untuk "create"/"update", insert new active row dengan snapshot lengkap.
// Best-effort: failure tidak block CRUD utama, cuma log warn.
func (s *ProductService) logTierChange(tier *entity.ProductPriceTier, action string, changedBy *string) {
	now := time.Now()
	// Close active row kalau action ∈ {update, delete} — sebelum insert versi baru
	// (atau jadi final state untuk delete).
	if action == "update" || action == "delete" {
		if err := s.TierHistoryRepo.CloseActive(tier.ID, now); err != nil {
			s.Log.Warn().Err(err).Str("tier_id", tier.ID).Str("action", action).Msg("tier history: close active failed")
		}
	}
	// Insert new row dengan snapshot. Untuk delete, snapshot pakai action=delete +
	// status=inactive + end_date=now, supaya history punya tombstone.
	memberIDsJSON := datatypes.JSON([]byte("null"))
	if len(tier.Members) > 0 {
		ids := make([]string, 0, len(tier.Members))
		for _, m := range tier.Members {
			ids = append(ids, m.ID)
		}
		if b, err := json.Marshal(ids); err == nil {
			memberIDsJSON = datatypes.JSON(b)
		}
	}
	row := &entity.ProductPriceTierHistory{
		ID:         uuid.New().String(),
		TierID:     tier.ID,
		ProductID:  tier.ProductID,
		MinQty:     tier.MinQty,
		Price:      tier.Price,
		TargetType: tier.TargetType,
		MemberIDs:  memberIDsJSON,
		Note:       tier.Note,
		Action:     action,
		StartDate:  now,
		ChangedBy:  changedBy,
	}
	if action == "delete" {
		row.Status = "inactive"
		row.EndDate = &now
	} else {
		row.Status = "active"
	}
	if err := s.TierHistoryRepo.Create(row); err != nil {
		s.Log.Warn().Err(err).Str("tier_id", tier.ID).Str("action", action).Msg("tier history: insert failed")
	}
}

// ListTierHistory returns audit trail untuk semua tier sebuah produk
// (termasuk tier yang sudah dihapus). Newest first.
func (s *ProductService) ListTierHistory(productID string) ([]dto.ProductPriceTierHistoryResponse, *dto.ApiError) {
	if _, err := s.Repo.FindByID(productID); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	rows, err := s.TierHistoryRepo.FindByProduct(productID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch tier history"}
	}
	out := make([]dto.ProductPriceTierHistoryResponse, 0, len(rows))
	for _, r := range rows {
		var memberIDs []string
		if len(r.MemberIDs) > 0 && string(r.MemberIDs) != "null" {
			_ = json.Unmarshal(r.MemberIDs, &memberIDs)
		}
		resp := dto.ProductPriceTierHistoryResponse{
			ID:         r.ID,
			TierID:     r.TierID,
			ProductID:  r.ProductID,
			MinQty:     r.MinQty,
			Price:      r.Price,
			TargetType: r.TargetType,
			MemberIDs:  memberIDs,
			Note:       r.Note,
			Status:     r.Status,
			Action:     r.Action,
			StartDate:  r.StartDate.Format(time.RFC3339),
			ChangedBy:  r.ChangedBy,
			CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		}
		if r.EndDate != nil {
			s := r.EndDate.Format(time.RFC3339)
			resp.EndDate = &s
		}
		out = append(out, resp)
	}
	return out, nil
}

func (s *ProductService) CreatePriceTier(productID string, req dto.SavePriceTierRequest, changedBy *string) (*dto.ProductPriceTierResponse, *dto.ApiError) {
	if _, err := s.Repo.FindByID(productID); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Product not found"}
	}
	tier, apiErr := s.buildTierFromRequest(productID, req)
	if apiErr != nil {
		return nil, apiErr
	}
	tier.ID = uuid.New().String()
	if err := s.TierRepo.Create(tier); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create tier")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create tier"}
	}
	// Refetch to get joined members with full names.
	created, _ := s.TierRepo.FindByID(tier.ID)
	if created != nil {
		tier = created
	}
	s.logTierChange(tier, "create", changedBy)
	resp := tierToResponse(tier)
	return &resp, nil
}

func (s *ProductService) UpdatePriceTier(tierID string, req dto.SavePriceTierRequest, changedBy *string) (*dto.ProductPriceTierResponse, *dto.ApiError) {
	existing, err := s.TierRepo.FindByID(tierID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Tier not found"}
	}
	updated, apiErr := s.buildTierFromRequest(existing.ProductID, req)
	if apiErr != nil {
		return nil, apiErr
	}
	updated.ID = tierID
	if err := s.TierRepo.Update(updated); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update tier")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update tier"}
	}
	refreshed, _ := s.TierRepo.FindByID(tierID)
	if refreshed != nil {
		updated = refreshed
	}
	s.logTierChange(updated, "update", changedBy)
	resp := tierToResponse(updated)
	return &resp, nil
}

func (s *ProductService) DeletePriceTier(tierID string, changedBy *string) *dto.ApiError {
	existing, err := s.TierRepo.FindByID(tierID)
	if err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Tier not found"}
	}
	// Snapshot SEBELUM delete supaya history punya last-known state.
	s.logTierChange(existing, "delete", changedBy)
	if err := s.TierRepo.Delete(tierID); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete tier")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete tier"}
	}
	return nil
}
