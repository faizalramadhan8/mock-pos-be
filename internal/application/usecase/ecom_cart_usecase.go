package usecase

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomCartService — cart operations untuk authenticated customer.
type EcomCartService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomCartService(ctx context.Context, db *gorm.DB) *EcomCartService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomCartService{DB: db, Log: logger}
}

// GetCart — return full cart dengan produk info + flag unavailable kalau
// produk sudah tidak tayang / stok 0 / dihapus.
func (s *EcomCartService) GetCart(userID string) (*dto.CartResponse, *dto.ApiError) {
	var items []entity.EcomCartItem
	if err := s.DB.Preload("Product").Where("user_id = ?", userID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
	}
	return s.buildResponse(items), nil
}

// AddItem — add produk ke cart. Kalau sudah ada, increment quantity.
// Validate: produk exists + ecom_is_available + stok cukup + minOrder.
func (s *EcomCartService) AddItem(userID string, req dto.CartAddRequest) (*dto.CartResponse, *dto.ApiError) {
	var product entity.Product
	// Penyaring `location = 'home'` dihapus 30 Okt 2026: keranjang sekarang
	// boleh berisi barang dari dua toko. Yang menjaga tetap `ecom_is_available`
	// di bawah — itu yang menentukan barang boleh dijual online, dan berlaku
	// sama untuk kedua toko.
	if err := s.DB.Where("id = ? AND deleted_at IS NULL", req.ProductID).First(&product).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak ditemukan"}
	}
	if !product.EcomIsAvailable {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Produk tidak tersedia di toko online"}
	}
	// Penjaga di TITIK MASUK keranjang. Tanpa ini, siapa pun yang tahu id
	// produk toko yang belum tayang bisa memasukkannya ke keranjang lewat
	// panggilan API langsung — dan memotong stok toko itu saat checkout.
	if !TempatBolehTayang(s.DB, product.Location) {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Produk tidak tersedia di toko online"}
	}
	if req.Quantity < product.EcomMinOrder {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Jumlah kurang dari minimum order"}
	}

	// Upsert: cari existing row, kalau ada increment; kalau tidak, insert baru.
	var existing entity.EcomCartItem
	err := s.DB.Where("user_id = ? AND product_id = ?", userID, req.ProductID).First(&existing).Error
	if err == nil {
		// Existing row — increment quantity
		newQty := existing.Quantity + req.Quantity
		if newQty > product.Stock {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Stok tidak cukup"}
		}
		existing.Quantity = newQty
		if err := s.DB.Save(&existing).Error; err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update cart"}
		}
	} else {
		if req.Quantity > product.Stock {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Stok tidak cukup"}
		}
		newItem := entity.EcomCartItem{
			ID:        uuid.New().String(),
			UserID:    userID,
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
		}
		if err := s.DB.Create(&newItem).Error; err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to add to cart"}
		}
	}

	return s.GetCart(userID)
}

// UpdateItem — set quantity absolut. quantity=0 → delete.
func (s *EcomCartService) UpdateItem(userID, itemID string, req dto.CartUpdateRequest) (*dto.CartResponse, *dto.ApiError) {
	var item entity.EcomCartItem
	if err := s.DB.Preload("Product").Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Item tidak ditemukan di cart"}
	}
	if req.Quantity == 0 {
		if err := s.DB.Delete(&item).Error; err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to remove item"}
		}
		return s.GetCart(userID)
	}
	if item.Product != nil {
		if req.Quantity < item.Product.EcomMinOrder {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Jumlah kurang dari minimum order"}
		}
		if req.Quantity > item.Product.Stock {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Stok tidak cukup"}
		}
	}
	item.Quantity = req.Quantity
	if err := s.DB.Save(&item).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update"}
	}
	return s.GetCart(userID)
}

// RemoveItem — delete row cart.
func (s *EcomCartService) RemoveItem(userID, itemID string) (*dto.CartResponse, *dto.ApiError) {
	if err := s.DB.Where("id = ? AND user_id = ?", itemID, userID).Delete(&entity.EcomCartItem{}).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to remove"}
	}
	return s.GetCart(userID)
}

// Clear — hapus semua cart items user. Dipakai setelah checkout success.
func (s *EcomCartService) Clear(userID string) *dto.ApiError {
	if err := s.DB.Where("user_id = ?", userID).Delete(&entity.EcomCartItem{}).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to clear cart"}
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────
func (s *EcomCartService) buildResponse(items []entity.EcomCartItem) *dto.CartResponse {
	resp := &dto.CartResponse{Items: []dto.CartItemResponse{}, Groups: []dto.CartGroup{}}
	perTempat := map[string]*dto.CartGroup{}
	for _, it := range items {
		if it.Product == nil {
			continue // FK cascade broke or race — skip silently
		}
		p := it.Product
		// Keranjang HARUS memakai penghitung yang sama dengan checkout,
		// termasuk harga grosir mode paket. Kalau dihitung sendiri di sini,
		// angka di keranjang berbeda dari yang ditagih — dan customer baru
		// sadar di layar pembayaran.
		lp := computeEcomLinePrice(s.DB, p, it.Quantity)
		price := lp.UnitPrice
		// Ecom image override (Sprint 5 gallery main image), fallback ke POS image.
		image := p.Image
		if p.EcomImage != nil && *p.EcomImage != "" {
			image = *p.EcomImage
		}
		subtotal := lp.LineTotal

		unavailable := false
		unavailableReason := ""
		if !p.EcomIsAvailable {
			unavailable = true
			unavailableReason = "Produk sudah tidak tersedia online"
		} else if p.Stock == 0 {
			unavailable = true
			unavailableReason = "Stok habis"
		} else if it.Quantity > p.Stock {
			unavailable = true
			unavailableReason = "Stok tidak cukup"
		}

		loc := TempatSah(p.Location)
		row := dto.CartItemResponse{
			Location:          loc,
			StoreName:         NamaToko(s.DB, loc),
			ID:                it.ID,
			ProductID:         p.ID,
			Name:              p.Name,
			NameID:            p.NameID,
			SKU:               p.SKU,
			Image:             image,
			Quantity:          it.Quantity,
			Price:             price,
			MemberPrice:       p.EcomMemberPrice,
			Stock:             p.Stock,
			MinOrder:          p.EcomMinOrder,
			WeightGrams:       p.EcomWeightGrams,
			Subtotal:          subtotal,
			Unavailable:       unavailable,
			UnavailableReason: unavailableReason,
		}
		resp.Items = append(resp.Items, row)
		g := perTempat[loc]
		if g == nil {
			g = &dto.CartGroup{
				Location:       loc,
				StoreName:      row.StoreName,
				Items:          []dto.CartItemResponse{},
				MinOrderAmount: s.minimalBelanja(loc),
			}
			perTempat[loc] = g
		}
		g.Items = append(g.Items, row)
		if !unavailable {
			resp.Subtotal += subtotal
			resp.TotalQty += it.Quantity
			g.Subtotal += subtotal
			g.TotalQty += it.Quantity
			if p.EcomWeightGrams != nil {
				resp.TotalWeight += *p.EcomWeightGrams * it.Quantity
			}
		}
		resp.ItemCount = len(resp.Items)
		if unavailable {
			resp.HasUnavailable = true
		}
	}
	// Urutan tetap: rumah dulu, lalu Bekasi. Iterasi map di Go acak, dan
	// urutan keranjang yang berubah tiap muat ulang membuat customer kira
	// isinya berubah.
	for _, loc := range []string{TempatRumah, TempatToko} {
		if g := perTempat[loc]; g != nil {
			resp.Groups = append(resp.Groups, *g)
		}
	}
	return resp
}

// minimalBelanja — ambang belanja satu toko. 0 = tanpa batas.
func (s *EcomCartService) minimalBelanja(location string) float64 {
	var st entity.EcomSettings
	if err := s.DB.Select("min_order_amount").Where("location = ?", TempatSah(location)).First(&st).Error; err != nil {
		return 0
	}
	return st.MinOrderAmount
}
