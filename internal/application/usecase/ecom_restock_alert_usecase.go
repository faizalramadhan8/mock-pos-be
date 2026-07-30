package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/email"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomRestockAlertService — Sprint 3 #16. Subscribe/unsubscribe alert +
// dispatch notif saat produk restock.
type EcomRestockAlertService struct {
	DB    *gorm.DB
	Log   *zerolog.Logger
	Email *email.Client
	Push  *PushService
	Cfg   *config.Config
}

func NewEcomRestockAlertService(ctx context.Context, db *gorm.DB) *EcomRestockAlertService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomRestockAlertService{
		DB:    db,
		Log:   logger,
		Email: email.NewClient(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName),
		Push:  NewPushService(ctx, db),
		Cfg:   cfg,
	}
}

// Subscribe — customer subscribe alert untuk 1 produk.
func (s *EcomRestockAlertService) Subscribe(userID, productID string) *dto.ApiError {
	// Cek produk exists (soft check — kalau tidak ada di ecom, tetap allow
	// subscribe supaya customer bisa jaga niat beli).
	var count int64
	s.DB.Model(&entity.Product{}).Where("id = ? AND deleted_at IS NULL", productID).Count(&count)
	if count == 0 {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Produk tidak ditemukan"}
	}

	// Upsert — kalau sudah ada, reset notified_at supaya bisa re-notify di
	// restock berikutnya. UNIQUE INDEX di DB cegah dup.
	row := entity.EcomRestockAlert{
		ID:        uuid.New().String(),
		UserID:    userID,
		ProductID: productID,
	}
	err := s.DB.Where("user_id = ? AND product_id = ?", userID, productID).First(&entity.EcomRestockAlert{}).Error
	if err == nil {
		// Sudah ada — cukup reset notified_at (siap notify lagi kalau restock).
		s.DB.Model(&entity.EcomRestockAlert{}).
			Where("user_id = ? AND product_id = ?", userID, productID).
			Update("notified_at", nil)
		return nil
	}
	if err := s.DB.Create(&row).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal subscribe"}
	}
	return nil
}

func (s *EcomRestockAlertService) Unsubscribe(userID, productID string) *dto.ApiError {
	if err := s.DB.Where("user_id = ? AND product_id = ?", userID, productID).
		Delete(&entity.EcomRestockAlert{}).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal unsubscribe"}
	}
	return nil
}

// IsSubscribed — cek untuk FE toggle state.
func (s *EcomRestockAlertService) IsSubscribed(userID, productID string) bool {
	var count int64
	s.DB.Model(&entity.EcomRestockAlert{}).
		Where("user_id = ? AND product_id = ?", userID, productID).
		Count(&count)
	return count > 0
}

// TriggerRestockNotif — dipanggil saat stok produk berubah dari 0 → >0.
// Ambil semua subscriber yang belum di-notify (notified_at IS NULL), kirim
// email + push, lalu mark notified_at. Best-effort per subscriber.
//
// Dipanggil dari ProductService.Update kalau stok naik dari 0. Wire di
// admin update stock atau saat cancel order (yang restore stok).
func (s *EcomRestockAlertService) TriggerRestockNotif(productID string) {
	if s.Email == nil {
		return
	}
	// Fetch produk untuk info.
	var product entity.Product
	if err := s.DB.Where("id = ?", productID).First(&product).Error; err != nil {
		return
	}
	// Skip kalau stock_ecom masih 0 (mungkin dispatch dipanggil salah).
	if product.StockEcom <= 0 {
		return
	}

	// Ambil subscriber yang belum notified.
	var alerts []entity.EcomRestockAlert
	if err := s.DB.Where("product_id = ? AND notified_at IS NULL", productID).Find(&alerts).Error; err != nil {
		s.Log.Warn().Err(err).Str("product_id", productID).Msg("restock notif: fetch subs failed")
		return
	}
	if len(alerts) == 0 {
		return
	}

	appURL := s.Cfg.AppURL
	if appURL == "" {
		appURL = "https://tbksanti.id"
	}
	productURL := strings.TrimRight(appURL, "/") + "/shop/produk/" + productID

	now := time.Now()
	for _, a := range alerts {
		var user entity.User
		if err := s.DB.Where("id = ?", a.UserID).First(&user).Error; err != nil {
			continue
		}

		// Send email
		if user.Email != "" {
			subject := fmt.Sprintf("%s sudah tersedia — TBK Santi", product.NameID)
			html := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:24px">
<h2 style="color:#C4302B;margin:0 0 8px">Kabar baik untukmu</h2>
<p style="color:#333;font-size:14px;line-height:1.6">
Hai %s, produk yang kamu tunggu <b>%s</b> sekarang sudah tersedia lagi.<br><br>
Stok masih terbatas, buruan checkout sebelum kehabisan.
</p>
<div style="margin:24px 0">
<a href="%s" style="display:inline-block;padding:12px 24px;background:#C4302B;color:#fff;border-radius:8px;text-decoration:none;font-weight:bold">Lihat Produk</a>
</div>
<p style="color:#999;font-size:12px;margin-top:32px">Toko Bahan Kue Santi · tbksanti.id</p>
</div>`, user.FullName, product.NameID, productURL)
			text := fmt.Sprintf("%s sudah tersedia. Cek: %s", product.NameID, productURL)
			s.Email.Send(user.Email, user.FullName, subject, html, text)
		}

		// Send push
		s.Push.SendToUser(a.UserID, "Produk yang kamu tunggu tersedia",
			fmt.Sprintf("%s sudah kembali — buruan checkout!", product.NameID), productURL)

		// Mark notified — cegah dobel kirim kalau function dipanggil ulang.
		s.DB.Model(&entity.EcomRestockAlert{}).
			Where("id = ?", a.ID).
			Update("notified_at", now)
	}
	s.Log.Info().Str("product_id", productID).Int("notified", len(alerts)).Msg("restock alert dispatched")
}
