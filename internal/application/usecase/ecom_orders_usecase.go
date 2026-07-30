package usecase

import (
	"context"
	"encoding/json"
	"fmt"
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
	DB    *gorm.DB
	Log   *zerolog.Logger
	Cfg   *config.Config
	PG    *pg.Client
	Admin *EcomAdminOrdersService // untuk reuse SendOrderStatusEmail
}

func NewEcomOrdersService(ctx context.Context, db *gorm.DB) *EcomOrdersService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomOrdersService{
		DB:    db,
		Log:   logger,
		Cfg:   cfg,
		PG:    pg.NewClient(cfg.PGBaseURL, cfg.PGAppKey, cfg.PGAppSecret, cfg.PGMerchantName),
		Admin: NewEcomAdminOrdersService(ctx, db),
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

// CancelPending — customer batalkan sendiri order yang masih pending_payment
// (belum bayar). Marketplace pattern: cegah customer harus WA admin kalau
// ganti pikiran sebelum bayar. Restore stock_ecom yang di-reserve saat
// CreateOrder. Post-paid orders TIDAK bisa cancel via endpoint ini (admin
// yang handle refund flow terpisah).
func (s *EcomOrdersService) CancelPending(userID, orderID string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	var order entity.Order
	if err := s.DB.Preload("Items").
		Where("id = ? AND ecom_user_id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID, userID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Pesanan tidak ditemukan"}
	}
	current := ""
	if order.EcomStatus != nil {
		current = *order.EcomStatus
	}
	if current != "pending_payment" {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Pesanan tidak bisa dibatalkan karena sudah " + current,
		}
	}

	tx := s.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Restore stock_ecom untuk semua item.
	for _, it := range order.Items {
		if it.ProductID == "" {
			continue
		}
		if err := tx.Model(&entity.Product{}).Where("id = ?", it.ProductID).
			Update("stock_ecom", gorm.Expr("stock_ecom + ?", it.Quantity)).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal restore stok"}
		}
	}
	updates := map[string]interface{}{
		"ecom_status": "cancelled",
		"status":      "cancelled",
	}
	if err := tx.Model(&entity.Order{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal simpan pembatalan"}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal commit"}
	}
	s.Log.Info().Str("order_id", orderID).Str("user_id", userID).Msg("customer cancelled pending order")
	return s.GetDetail(userID, orderID)
}

// RetryPayment — regenerate payment_url di PG DOKU untuk order pending_payment
// yang link-nya sudah expired (24 jam dari CreateOrder). Reuse trx_id supaya
// PG side idempotent (kalau sudah paid, PG return status PAID → kita sync).
// Kalau expired real, PG generate URL baru.
func (s *EcomOrdersService) RetryPayment(userID, orderID string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	var order entity.Order
	if err := s.DB.Preload("Items").
		Where("id = ? AND ecom_user_id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID, userID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Pesanan tidak ditemukan"}
	}
	current := ""
	if order.EcomStatus != nil {
		current = *order.EcomStatus
	}
	if current != "pending_payment" {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Retry hanya untuk pesanan menunggu pembayaran",
		}
	}
	if !s.PG.IsConfigured() {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Gateway pembayaran belum siap"}
	}

	// Cek status di PG dulu — kalau sudah PAID, tinggal sync (jangan
	// regenerate → double charge risk).
	pgCtx, pgCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pgCancel()
	if statusResp, err := s.PG.CheckStatus(pgCtx, orderID); err == nil &&
		strings.ToUpper(statusResp.PaymentStatus) == "PAID" {
		s.Log.Info().Str("order_id", orderID).Msg("RetryPayment: PG sudah PAID, sync ke DB")
		s.syncPaymentStatusFromPG(&order)
		return s.GetDetail(userID, orderID)
	}

	// Ambil user info untuk billing.
	var user entity.User
	if err := s.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "User tidak ditemukan"}
	}
	// Address snapshot untuk billing recipient.
	billingName := user.FullName
	billingPhone := user.PhoneNumber
	if order.ShippingAddressSnapshot != nil {
		var snap map[string]interface{}
		if json.Unmarshal([]byte(*order.ShippingAddressSnapshot), &snap) == nil {
			if v, ok := snap["recipient_name"].(string); ok && v != "" {
				billingName = v
			}
			if v, ok := snap["recipient_phone"].(string); ok && v != "" {
				billingPhone = v
			}
		}
	}

	// Rebuild items.
	pgItems := make([]pg.CreatePaymentItem, 0, len(order.Items))
	for _, it := range order.Items {
		pgItems = append(pgItems, pg.CreatePaymentItem{
			ProductID: it.ProductID, ProductCode: it.ProductID, ProductName: it.Name,
			Price: it.UnitPrice, Quantity: it.Quantity,
		})
	}
	if order.ShippingCost > 0 {
		pgItems = append(pgItems, pg.CreatePaymentItem{
			ProductID: "shipping", ProductCode: "shipping",
			ProductName: "Ongkir", Price: order.ShippingCost, Quantity: 1,
		})
	}

	channel := ""
	if order.PaymentChannel != nil {
		channel = *order.PaymentChannel
	}
	if channel == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Channel pembayaran tidak tersimpan"}
	}
	merchant := s.PG.MerchantName
	if merchant == "" {
		merchant = "Testing"
	}
	callbackURL := strings.TrimRight(s.Cfg.AppURL, "/") + "/api/v1/ecom/payments/webhook/pg"
	successURL := strings.TrimRight(s.Cfg.AppURL, "/") + "/shop/pesanan/" + orderID + "?paid=1"

	// Cegah trx_id conflict di PG — append suffix retry. PG treat sebagai
	// transaksi baru, tapi mapping tetap ke order.id via reference/query.
	// (Trade-off: kalau PG punya idempotency check by trx_id, retry gagal.
	//  Untuk sekarang assume PG treat sebagai new).
	retryTrxID := orderID + "-r" + time.Now().Format("20060102150405")

	pgReq := pg.CreatePaymentRequest{
		Channel:        channel,
		TotalAmount:    order.Total,
		TrxID:          retryTrxID,
		BillingEmail:   user.Email,
		BillingName:    billingName,
		BillingPhone:   billingPhone,
		BillingAddress: "",
		BillingID:      userID,
		BillingUID:     userID,
		CallbackURL:    callbackURL,
		SuccessURL:     successURL,
		Description:    fmt.Sprintf("Retry Order #%s", orderID[:8]),
		Merchant:       merchant,
		Remark:         "retry",
		Item:           pgItems,
	}

	retryCtx, retryCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer retryCancel()
	pgResp, err := s.PG.CreatePayment(retryCtx, pgReq)
	if err != nil {
		s.Log.Error().Err(err).Str("order_id", orderID).Msg("RetryPayment: PG create failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadGateway, Message: "Gagal buat link bayar baru: " + err.Error()}
	}
	if pgResp.PaymentURL == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadGateway, Message: "Gateway tidak mengirim URL bayar"}
	}

	// Update payment_url + payment_reference baru + geser expired_at 24 jam.
	newExpired := time.Now().Add(24 * time.Hour)
	updates := map[string]interface{}{
		"payment_url":        pgResp.PaymentURL,
		"payment_reference":  pgResp.PaymentID,
		"payment_expired_at": newExpired,
	}
	if err := s.DB.Model(&entity.Order{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
		s.Log.Error().Err(err).Str("order_id", orderID).Msg("RetryPayment: save url failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Link bayar dibuat tapi gagal simpan"}
	}
	s.Log.Info().Str("order_id", orderID).Str("retry_trx", retryTrxID).Msg("RetryPayment: URL regenerated")
	return s.GetDetail(userID, orderID)
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
	// Fire email "pesanan selesai + tulis ulasan" — async, best-effort.
	go s.Admin.SendOrderStatusEmail(orderID, "completed")
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

	// Shipping — prefer *_name (display friendly) untuk FE, fallback ke
	// code kalau kolom baru NULL (order lama pre-migration 000061).
	if order.ShippingCourierName != nil && *order.ShippingCourierName != "" {
		detail.Shipping.Courier = *order.ShippingCourierName
	} else if order.ShippingCourier != nil {
		detail.Shipping.Courier = *order.ShippingCourier
	}
	if order.ShippingServiceName != nil && *order.ShippingServiceName != "" {
		detail.Shipping.ServiceName = *order.ShippingServiceName
	} else if order.ShippingService != nil {
		detail.Shipping.ServiceName = *order.ShippingService
	}
	if order.ShippingETD != nil {
		detail.Shipping.ETD = *order.ShippingETD
	}
	if order.ShippingAWB != nil {
		detail.Shipping.AWB = *order.ShippingAWB
	}
	if order.ShippingTrackingURL != nil {
		detail.Shipping.TrackingURL = *order.ShippingTrackingURL
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
