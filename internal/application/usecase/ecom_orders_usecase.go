package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/pg"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomOrdersService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
	Cfg *config.Config
	PG  *pg.Client
}

func NewEcomOrdersService(ctx context.Context, db *gorm.DB) *EcomOrdersService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomOrdersService{
		DB:  db,
		Log: logger,
		Cfg: cfg,
		PG:  pg.NewClient(cfg.PGBaseURL, cfg.PGAppKey, cfg.PGAppSecret, cfg.PGMerchantName),
	}
}

// midtransSnapURL — build hosted checkout URL dari snap_token + env.
// Format v3/redirection = Snap page terbaru (v2/vtweb deprecated).
// Sandbox vs Prod ditentukan dari MIDTRANS_IS_PROD (bukan dari isi token —
// token tidak punya prefix yang bisa di-detect).
// isStubToken — token yang di-generate saat Midtrans belum configured.
// FE tidak render tombol "Bayar Sekarang" untuk order dengan stub token.
func isStubToken(t string) bool {
	return len(t) >= 5 && t[:5] == "stub-"
}

func (s *EcomOrdersService) midtransSnapURL(token string) string {
	if token == "" {
		return ""
	}
	base := "https://app.sandbox.midtrans.com"
	if s.Cfg.MidtransIsProd {
		base = "https://app.midtrans.com"
	}
	return base + "/snap/v3/redirection/" + token
}

func (s *EcomOrdersService) List(userID string) ([]dto.CustomerOrderListItem, *dto.ApiError) {
	var orders []entity.Order
	if err := s.DB.Preload("Items").
		Where("ecom_user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch orders"}
	}

	// Self-heal pending_payment orders — ping PG untuk yang recent (< 24 jam,
	// cegah ping expired trx). Cap 5 per request supaya list load tidak lambat
	// kalau customer punya banyak order pending (rare). Serial call di
	// goroutine paralel tidak perlu — 5-order latency masih < 5s worst case.
	pendingCount := 0
	for i := range orders {
		if orders[i].EcomStatus == nil || *orders[i].EcomStatus != "pending_payment" {
			continue
		}
		if time.Since(orders[i].CreatedAt) > 24*time.Hour {
			continue
		}
		s.syncPaymentStatusFromPG(&orders[i])
		pendingCount++
		if pendingCount >= 5 {
			break
		}
	}

	out := make([]dto.CustomerOrderListItem, 0, len(orders))
	for _, o := range orders {
		ecomStatus := ""
		if o.EcomStatus != nil {
			ecomStatus = *o.EcomStatus
		}
		firstItem := ""
		if len(o.Items) > 0 {
			firstItem = o.Items[0].Name
		}
		row := dto.CustomerOrderListItem{
			ID:        o.ID,
			Total:     o.Total,
			EcomStatus: ecomStatus,
			ItemCount: len(o.Items),
			FirstItem: firstItem,
			CreatedAt: o.CreatedAt.Format(time.RFC3339),
		}
		if o.PaymentPaidAt != nil {
			s := o.PaymentPaidAt.Format(time.RFC3339)
			row.PaymentPaidAt = &s
		}
		out = append(out, row)
	}
	return out, nil
}

// ConfirmReceived — customer klik "Barang Diterima" di app. Transisi
// shipped|delivered → completed. Cegah customer transisi status yang bukan
// haknya (mis. mark cancelled) dengan hardcode target = "completed" di sini.
//
// Marketplace pattern (Tokopedia/Shopee/Blibli): dana toko dilepas dari escrow
// baru setelah customer konfirmasi. Di kita dana Midtrans langsung masuk
// rekening Bu Santi jadi tidak ada escrow, tapi flow ini masih guna:
//   - Gate review (cuma yang sudah konfirmasi terima yang bisa review)
//   - Confirmation window untuk customer complain sebelum "final"
//   - Audit data akurat: terima ≠ kurir tag delivered
func (s *EcomOrdersService) ConfirmReceived(userID, orderID string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	var order entity.Order
	if err := s.DB.Where("id = ? AND ecom_user_id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID, userID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}
	current := ""
	if order.EcomStatus != nil {
		current = *order.EcomStatus
	}
	if current != "shipped" && current != "delivered" {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Belum bisa konfirmasi terima — status saat ini: " + current,
		}
	}
	updates := map[string]interface{}{
		"ecom_status": "completed",
		"status":      "completed", // sync POS-side (mirror UpdateStatus admin)
	}
	if err := s.DB.Model(&entity.Order{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
		s.Log.Error().Err(err).Str("order_id", orderID).Msg("customer confirm received: update failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan konfirmasi"}
	}
	s.Log.Info().Str("order_id", orderID).Str("user_id", userID).Str("from", current).Msg("customer confirmed order received")
	return s.GetDetail(userID, orderID)
}

// syncPaymentStatusFromPG — best-effort re-sync order status dari PG DOKU.
// Dipanggil di GetDetail tiap customer buka halaman detail order, sebab PG
// wrapper alifworks pakai redirect (bukan async webhook) — kalau customer
// close browser sebelum redirect, DB kita stuck di pending_payment.
//
// Cegah race: cuma sync kalau ecom_status='pending_payment' + ada
// payment_reference (CreatePayment sudah sukses sebelumnya). Kalau PG
// return PAID, update DB + fire email best-effort. Failure di sini tidak
// blok display order — sekadar log warn.
func (s *EcomOrdersService) syncPaymentStatusFromPG(order *entity.Order) {
	if order.EcomStatus == nil || *order.EcomStatus != "pending_payment" {
		return
	}
	if !s.PG.IsConfigured() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.PG.CheckStatus(ctx, order.ID)
	if err != nil {
		s.Log.Warn().Err(err).Str("order_id", order.ID).Msg("PG CheckStatus sync failed")
		return
	}
	status := strings.ToUpper(resp.PaymentStatus)
	if status != "PAID" {
		return
	}
	now := time.Now()
	updates := map[string]interface{}{
		"ecom_status":     "paid",
		"status":          "completed",
		"payment_paid_at": now,
	}
	if err := s.DB.Model(&entity.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
		s.Log.Error().Err(err).Str("order_id", order.ID).Msg("PG sync: update order failed")
		return
	}
	s.Log.Info().Str("order_id", order.ID).Msg("PG sync: order auto-marked paid via CheckStatus")
	// Reflect ke local struct supaya response fresh — cegah customer lihat
	// stale "pending_payment" di refresh berikutnya.
	paidStatus := "paid"
	order.EcomStatus = &paidStatus
	order.Status = "completed"
	order.PaymentPaidAt = &now
}

func (s *EcomOrdersService) GetDetail(userID, orderID string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	var order entity.Order
	if err := s.DB.Preload("Items").
		Where("id = ? AND ecom_user_id = ? AND deleted_at IS NULL", orderID, userID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}

	// Self-heal: kalau order pending_payment, ping PG untuk cek status real.
	// Cover kasus customer sudah bayar tapi PG belum trigger webhook / callback
	// belum masuk (browser close, network down, dsb).
	s.syncPaymentStatusFromPG(&order)

	ecomStatus := ""
	if order.EcomStatus != nil {
		ecomStatus = *order.EcomStatus
	}

	detail := &dto.CustomerOrderDetail{
		ID:           order.ID,
		Subtotal:     order.Subtotal,
		ShippingCost: order.ShippingCost,
		Total:        order.Total,
		EcomStatus:   ecomStatus,
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
	}
	if order.EcomDeliveredAt != nil {
		dstr := order.EcomDeliveredAt.Format(time.RFC3339)
		detail.EcomDeliveredAt = &dstr
	}

	// Items
	for _, it := range order.Items {
		detail.Items = append(detail.Items, dto.CustomerOrderItemDetail{
			ProductID: it.ProductID,
			Name:      it.Name,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			Subtotal:  it.UnitPrice * float64(it.Quantity),
		})
	}

	// Shipping
	if order.ShippingCourier != nil {
		detail.Shipping.Courier = *order.ShippingCourier
	}
	if order.ShippingService != nil {
		detail.Shipping.ServiceName = *order.ShippingService
	}
	if order.ShippingETD != nil {
		detail.Shipping.ETD = *order.ShippingETD
	}
	if order.ShippingAWB != nil {
		detail.Shipping.AWB = *order.ShippingAWB
	}
	if order.BiteshipOrderID != nil {
		detail.Shipping.BiteshipOrderID = *order.BiteshipOrderID
	}

	// Address snapshot (decode JSON)
	if order.ShippingAddressSnapshot != nil {
		var snap map[string]interface{}
		if err := json.Unmarshal([]byte(*order.ShippingAddressSnapshot), &snap); err == nil {
			get := func(k string) string {
				if v, ok := snap[k].(string); ok {
					return v
				}
				return ""
			}
			detail.Shipping.Address.Label = get("label")
			detail.Shipping.Address.RecipientName = get("recipient_name")
			detail.Shipping.Address.RecipientPhone = get("recipient_phone")
			detail.Shipping.Address.StreetAddress = get("street_address")
			detail.Shipping.Address.Subdistrict = get("subdistrict")
			detail.Shipping.Address.District = get("district")
			detail.Shipping.Address.City = get("city")
			detail.Shipping.Address.Province = get("province")
			detail.Shipping.Address.Zipcode = get("zipcode")
			detail.Shipping.Address.Notes = get("notes")
		}
	}

	// Payment — PG DOKU (28 Jul 2026). Kalau payment_url ke-set dan bukan
	// stub, tampil sebagai mode="pg" dengan link ke DOKU checkout. Kalau
	// kosong/stub = manual (customer transfer bank + hubungi admin).
	// Order lama Midtrans yang punya payment_snap_token tapi payment_url
	// kosong dianggap manual — Bu Santi harus verify manual.
	if order.PaymentURL != nil && *order.PaymentURL != "" && !isStubToken(*order.PaymentURL) {
		detail.Payment.Mode = "pg"
		detail.Payment.PaymentURL = *order.PaymentURL
	} else {
		detail.Payment.Mode = "manual"
	}
	if order.PaymentChannel != nil {
		detail.Payment.Channel = *order.PaymentChannel
	}
	if order.PaymentChannelCategory != nil {
		detail.Payment.ChannelCategory = *order.PaymentChannelCategory
	}
	if order.PaymentReference != nil {
		detail.Payment.Reference = *order.PaymentReference
	}
	if order.PaymentPaidAt != nil {
		s := order.PaymentPaidAt.Format(time.RFC3339)
		detail.Payment.PaidAt = &s
	}
	if order.PaymentExpiredAt != nil {
		s := order.PaymentExpiredAt.Format(time.RFC3339)
		detail.Payment.ExpiredAt = &s
	}

	return detail, nil
}
