package usecase

import (
	"context"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Katalog produk toko rumah — dibuat dan dikelola dari panel admin rumah.
//
// Sebelum 21 Okt 2026 produk hanya bisa lahir di POS; panel ecom cuma bisa
// mengubah kolom `ecom_*`. Begitu tiap tempat punya katalognya sendiri
// (migration 000075), panel rumah harus bisa membuat produknya sendiri —
// kalau tidak, Bu Santi harus membuat produk rumah di POS lalu produknya
// langsung lenyap dari sana, dan nama atau harganya tidak bisa diubah lagi
// dari mana pun.
//
// Logika pembuatan/pengubahannya SENGAJA memakai ProductService yang sama
// dengan POS. Menyalinnya berarti dua tempat yang harus diingat setiap kali
// aturan produk berubah — riwayat harga, movement stok awal, validasi SKU.
type EcomAdminCatalogService struct {
	DB        *gorm.DB
	Product   *ProductService
	Inventory *InventoryService
}

func NewEcomAdminCatalogService(ctx context.Context, db *gorm.DB) *EcomAdminCatalogService {
	// Keduanya dibangun dengan ctx SUNGGUHAN. NewInventoryService menegaskan
	// tipe logger dari ctx (`ctx.Value(LoggerCtxKey).(*zerolog.Logger)`),
	// jadi memanggilnya dengan context.Background() akan panic — bukan error
	// yang bisa ditangani, tapi backend mati — pada percobaan pertama
	// menyesuaikan stok rumah.
	return &EcomAdminCatalogService{
		DB:        db,
		Product:   NewProductService(ctx, db),
		Inventory: NewInventoryService(ctx, db),
	}
}

// pastikanMilikRumah — penjaga kepemilikan.
//
// Panel rumah hanya boleh menyentuh produknya sendiri. Tanpa ini, siapa pun
// yang tahu id produk Bekasi bisa mengubah atau menghapusnya lewat panel
// rumah — dan perubahannya akan muncul di kasir tanpa jejak di POS.
func (s *EcomAdminCatalogService) pastikanMilikRumah(id string) *dto.ApiError {
	var p entity.Product
	if err := s.DB.Select("id", "location").
		Where("id = ? AND deleted_at IS NULL", id).First(&p).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak ditemukan"}
	}
	if p.Location != "home" {
		// Pesannya sengaja tidak membocorkan bahwa produknya ada di Bekasi —
		// dari sudut pandang panel rumah, produk itu memang tidak ada.
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak ditemukan"}
	}
	return nil
}

// Create — produk baru milik toko rumah.
func (s *EcomAdminCatalogService) Create(req dto.CreateProductRequest, userID string) (*dto.ProductResponse, *dto.ApiError) {
	// Dipaksa, bukan dibaca dari request. Panel rumah tidak boleh bisa
	// menitipkan produk ke katalog Bekasi.
	req.Location = "home"
	return s.Product.Create(req, userID)
}

// Update — ubah produk rumah. `location` tidak ikut berubah: memindahkan
// produk antar katalog sama dengan memindahkan stok antar tempat tanpa
// jejak, dan itu harus lewat opname di masing-masing tempat.
func (s *EcomAdminCatalogService) Update(id string, req dto.UpdateProductRequest, userID string) (*dto.ProductResponse, *dto.ApiError) {
	if fail := s.pastikanMilikRumah(id); fail != nil {
		return nil, fail
	}
	return s.Product.Update(id, req, userID)
}

func (s *EcomAdminCatalogService) Delete(id string) *dto.ApiError {
	if fail := s.pastikanMilikRumah(id); fail != nil {
		return fail
	}
	return s.Product.Delete(id)
}

// AdjustStock — satu-satunya jalur ubah stok rumah, sama seperti di POS:
// wajib alasan, set absolut, menulis `stock_movements`. Tempatnya diambil
// dari produknya sendiri, jadi tidak ada pilihan yang bisa salah.
func (s *EcomAdminCatalogService) AdjustStock(id string, req dto.StockAdjustmentRequest, userID string) (*dto.StockMovementResponse, *dto.ApiError) {
	if fail := s.pastikanMilikRumah(id); fail != nil {
		return nil, fail
	}
	return s.Inventory.AdjustStock(id, req, userID)
}


// ─── Harga grosir (tier) produk rumah ─────────────────────────────────────
//
// Layar pengelola tier sebelumnya hanya ada di POS, dan POS kini menyaring
// produk rumah keluar — jadi produk rumah tidak bisa diberi harga kartonan
// sama sekali. Padahal storefront SUDAH menerapkannya
// (`computeEcomLinePrice`), dan Bu Santi menyebut pembelian kartonan sebagai
// tujuan toko rumah.
//
// Semua jalur di bawah memakai ProductService yang sama dengan POS, jadi
// riwayat perubahan tier (`product_price_tier_history`) tetap tercatat.
// Yang ditambahkan hanya penjaga kepemilikan.

// tierMilikRumah — resolusi tier → produk → tempat.
//
// Update dan Delete menerima tierID, bukan productID, jadi kepemilikannya
// harus ditelusuri lewat tiernya. Tanpa ini panel rumah bisa mengubah atau
// menghapus tier milik produk Bekasi kalau id tiernya diketahui — dan
// perubahannya langsung mengubah harga di konter kasir.
func (s *EcomAdminCatalogService) tierMilikRumah(tierID string) *dto.ApiError {
	var t entity.ProductPriceTier
	if err := s.DB.Select("id", "product_id").
		Where("id = ? AND deleted_at IS NULL", tierID).First(&t).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Tier tidak ditemukan"}
	}
	return s.pastikanMilikRumah(t.ProductID)
}

func (s *EcomAdminCatalogService) ListTiers(productID string) ([]dto.ProductPriceTierResponse, *dto.ApiError) {
	if fail := s.pastikanMilikRumah(productID); fail != nil {
		return nil, fail
	}
	return s.Product.ListPriceTiers(productID)
}

// paksaTargetSemuaCustomer — toko rumah tidak punya konsep member POS.
//
// `member_specific` menunjuk id di tabel `members` (member kasir Bekasi),
// sementara customer storefront adalah baris `users` dengan role 'user' —
// dua himpunan yang sama sekali berbeda. Tier member_specific pada produk
// rumah karena itu tidak akan pernah cocok dengan siapa pun, dan storefront
// memang hanya membaca `all_customers`. Dipaksa di sini supaya tidak ada
// tier yang tersimpan tapi tidak pernah berlaku.
func paksaTargetSemuaCustomer(req dto.SavePriceTierRequest) dto.SavePriceTierRequest {
	req.TargetType = "all_customers"
	req.MemberIDs = nil
	return req
}

func (s *EcomAdminCatalogService) CreateTier(productID string, req dto.SavePriceTierRequest, by *string) (*dto.ProductPriceTierResponse, *dto.ApiError) {
	if fail := s.pastikanMilikRumah(productID); fail != nil {
		return nil, fail
	}
	return s.Product.CreatePriceTier(productID, paksaTargetSemuaCustomer(req), by)
}

func (s *EcomAdminCatalogService) UpdateTier(tierID string, req dto.SavePriceTierRequest, by *string) (*dto.ProductPriceTierResponse, *dto.ApiError) {
	if fail := s.tierMilikRumah(tierID); fail != nil {
		return nil, fail
	}
	return s.Product.UpdatePriceTier(tierID, paksaTargetSemuaCustomer(req), by)
}

func (s *EcomAdminCatalogService) DeleteTier(tierID string, by *string) *dto.ApiError {
	if fail := s.tierMilikRumah(tierID); fail != nil {
		return fail
	}
	return s.Product.DeletePriceTier(tierID, by)
}
