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
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/email"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/midtrans"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomCheckoutService struct {
	DB       *gorm.DB
	Log      *zerolog.Logger
	Midtrans *midtrans.Client
	Email    *email.Client
	Voucher  *EcomVoucherService
	AppURL   string
}

func NewEcomCheckoutService(ctx context.Context, db *gorm.DB) *EcomCheckoutService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomCheckoutService{
		DB:       db,
		Log:      logger,
		Midtrans: midtrans.NewClient(cfg.MidtransServerKey, cfg.MidtransIsProd),
		Email:    email.NewClient(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName),
		Voucher:  NewEcomVoucherService(ctx, db),
		AppURL:   cfg.AppURL,
	}
}

// CreateOrder — end-to-end checkout: validate cart + address, create Order,
// decrement stock_ecom (reserve), clear cart, request Snap token.
// Kalau Midtrans belum configured, mode "manual" (customer transfer bank
// manual, Bu Santi verify).
// CartSubtotal — compute total dari cart user pakai harga effective ecom.
// Dipakai voucher validate endpoint supaya FE tidak perlu kirim subtotal
// (yang bisa di-tamper). Kalau selectedIDs non-empty, hanya sum item itu.
func (s *EcomCheckoutService) CartSubtotal(userID string, selectedIDs []string) (float64, *dto.ApiError) {
	var cart []entity.EcomCartItem
	q := s.DB.Preload("Product").Where("user_id = ?", userID)
	if len(selectedIDs) > 0 {
		q = q.Where("id IN ?", selectedIDs)
	}
	if err := q.Find(&cart).Error; err != nil {
		return 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
	}
	sub := 0.0
	for _, c := range cart {
		if c.Product == nil {
			continue
		}
		price := c.Product.SellingPrice
		if c.Product.EcomPrice != nil {
			price = *c.Product.EcomPrice
		}
		sub += price * float64(c.Quantity)
	}
	return sub, nil
}

func (s *EcomCheckoutService) CreateOrder(userID string, userEmail, userPhone, userName string, req dto.CheckoutCreateRequest) (*dto.CheckoutCreateResponse, *dto.ApiError) {
	// Load address (verify owner).
	var addr entity.EcomAddress
	if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.AddressID, userID).First(&addr).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Alamat tidak ditemukan"}
	}

	// Load cart (with product) — begin tx supaya stock reserve + order create atomic.
	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to start tx"}
	}

	// Build "checkout lines" — bisa dari 3 source (mutually exclusive):
	//   1. BuyNowItems (buy-now dari PDP) — bypass cart entirely
	//   2. SelectedItemIDs (partial cart checkout)
	//   3. Default: seluruh cart (backward compat)
	type checkoutLine struct {
		Product  *entity.Product
		Quantity int
		CartID   string // "" kalau buy-now
	}
	lines := []checkoutLine{}

	if len(req.BuyNowItems) > 0 {
		// Buy-now — load produk berdasar ID di request. Cart TIDAK disentuh.
		for _, bn := range req.BuyNowItems {
			var p entity.Product
			if err := tx.Where("id = ? AND deleted_at IS NULL", bn.ProductID).First(&p).Error; err != nil {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Produk tidak ditemukan"}
			}
			lines = append(lines, checkoutLine{Product: &p, Quantity: bn.Quantity})
		}
	} else {
		// Cart-based (full atau partial via SelectedItemIDs).
		var cart []entity.EcomCartItem
		q := tx.Preload("Product").Where("user_id = ?", userID)
		if len(req.SelectedItemIDs) > 0 {
			q = q.Where("id IN ?", req.SelectedItemIDs)
		}
		if err := q.Find(&cart).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
		}
		if len(cart) == 0 {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Tidak ada item yang dipilih"}
		}
		for _, c := range cart {
			lines = append(lines, checkoutLine{Product: c.Product, Quantity: c.Quantity, CartID: c.ID})
		}
	}

	// Validate + compute subtotal + reserve stock (atomic per line).
	subtotal := 0.0
	orderID := uuid.New().String()
	orderItems := make([]entity.OrderItem, 0, len(lines))
	consumedCartIDs := make([]string, 0, len(lines))
	for _, ln := range lines {
		p := ln.Product
		if p == nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Produk tidak valid"}
		}
		if !p.EcomIsAvailable {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Produk %s tidak tersedia", p.NameID)}
		}
		if ln.Quantity > p.StockEcom {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Stok %s tidak cukup (tersisa %d)", p.NameID, p.StockEcom)}
		}
		price := p.SellingPrice
		if p.EcomPrice != nil {
			price = *p.EcomPrice
		}
		lineTotal := price * float64(ln.Quantity)
		subtotal += lineTotal

		orderItems = append(orderItems, entity.OrderItem{
			ID:            uuid.New().String(),
			OrderID:       orderID,
			ProductID:     p.ID,
			Name:          p.NameID,
			Quantity:      ln.Quantity,
			UnitType:      "individual",
			UnitPrice:     price,
			PurchasePrice: p.PurchasePrice,
		})

		// Reserve stock — decrement stock_ecom (POS stock tidak affected).
		if err := tx.Model(&entity.Product{}).Where("id = ?", p.ID).
			Update("stock_ecom", gorm.Expr("stock_ecom - ?", ln.Quantity)).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to reserve stock"}
		}
		if ln.CartID != "" {
			consumedCartIDs = append(consumedCartIDs, ln.CartID)
		}
	}

	// Voucher — apply kalau ada kode. Discount di-cap ke subtotal (voucher
	// tidak boleh > subtotal, jadi customer minimal bayar ongkir).
	voucherDiscount := 0.0
	var voucherCodePtr *string
	if req.VoucherCode != "" {
		vd, v, verr := s.Voucher.ApplyOnCheckout(tx, req.VoucherCode, subtotal)
		if verr != nil {
			tx.Rollback()
			return nil, verr
		}
		voucherDiscount = vd
		if v != nil {
			c := v.Code
			voucherCodePtr = &c
		}
	}

	total := subtotal - voucherDiscount + req.ShippingCost
	if total < 0 {
		total = 0
	}

	// Snapshot alamat ke JSON.
	addrSnapshotBytes, _ := json.Marshal(map[string]interface{}{
		"label":           addr.Label,
		"recipient_name":  addr.RecipientName,
		"recipient_phone": addr.RecipientPhone,
		"province":        addr.Province,
		"city":            addr.City,
		"district":        addr.District,
		"subdistrict":     addr.Subdistrict,
		"zipcode":         addr.Zipcode,
		"street_address":  addr.StreetAddress,
		"notes":           addr.Notes,
	})
	addrSnapshot := string(addrSnapshotBytes)

	ecomStatus := "pending_payment"
	now := time.Now()
	expireAt := now.Add(24 * time.Hour) // 24 jam expire kalau tidak bayar

	// Create Order — pakai schema existing POS Order dengan ecom fields di-populate.
	// status='pending' (POS style) supaya tidak break existing Reports/Cashflow
	// yang filter WHERE status='completed'. Ecom-specific = pakai ecom_status.
	shippingCourierPtr := &req.ShippingCourier
	shippingServicePtr := &req.ShippingService
	shippingETDPtr := &req.ShippingETD
	ecomUserIDPtr := &userID
	ecomStatusPtr := &ecomStatus

	order := entity.Order{
		ID:                      orderID,
		Items:                   orderItems,
		Subtotal:                subtotal,
		PPNRate:                 0,
		PPN:                     0,
		Total:                   total,
		Payment:                 "pending",
		Status:                  "pending",
		Customer:                userName,
		CustomerPhone:           userPhone,
		CreatedBy:               userID,
		OrderSource:             "ecom",
		EcomUserID:              ecomUserIDPtr,
		ShippingAddressSnapshot: &addrSnapshot,
		ShippingCourier:         shippingCourierPtr,
		ShippingService:         shippingServicePtr,
		ShippingCost:            req.ShippingCost,
		ShippingETD:             shippingETDPtr,
		EcomStatus:              ecomStatusPtr,
		PaymentExpiredAt:        &expireAt,
		VoucherCode:             voucherCodePtr,
		VoucherDiscount:         voucherDiscount,
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create order"}
	}

	// Request Midtrans Snap token (atau stub kalau not configured).
	itemDetails := make([]midtrans.ItemDetail, 0, len(orderItems)+1)
	for _, it := range orderItems {
		itemDetails = append(itemDetails, midtrans.ItemDetail{
			ID: it.ProductID, Name: it.Name, Price: it.UnitPrice, Quantity: it.Quantity,
		})
	}
	if req.ShippingCost > 0 {
		itemDetails = append(itemDetails, midtrans.ItemDetail{
			ID: "shipping", Name: fmt.Sprintf("Ongkir %s %s", req.ShippingCourier, req.ShippingService),
			Price: req.ShippingCost, Quantity: 1,
		})
	}

	snapReq := midtrans.SnapRequest{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:     orderID,
			GrossAmount: total,
		},
		ItemDetails: itemDetails,
		CustomerDetails: &midtrans.CustomerDetail{
			FirstName: addr.RecipientName,
			Email:     userEmail,
			Phone:     addr.RecipientPhone,
		},
		Expiry: &midtrans.SnapExpiry{
			StartTime: now.Format("2006-01-02 15:04:05 -0700"),
			Unit:      "hour",
			Duration:  24,
		},
	}

	snapResp, snapErr := s.Midtrans.CreateSnapToken(snapReq)
	if snapErr != nil {
		s.Log.Warn().Err(snapErr).Str("order_id", orderID).Msg("Snap token failed, fallback manual")
	}

	paymentMode := "midtrans"
	snapToken := ""
	snapRedirectURL := ""
	if s.Midtrans.IsConfigured() && snapResp != nil {
		snapToken = snapResp.Token
		snapRedirectURL = snapResp.RedirectURL
	} else {
		paymentMode = "manual"
	}
	if snapToken != "" {
		// Save snap_token ke order supaya bisa retry payment nanti.
		if err := tx.Model(&entity.Order{}).Where("id = ?", orderID).
			Update("payment_snap_token", snapToken).Error; err != nil {
			s.Log.Warn().Err(err).Msg("Failed to save snap_token")
		}
	}

	// Clear ONLY consumed cart items — item lain tetap tersimpan untuk
	// checkout berikutnya (partial cart pattern). Buy-now (consumedCartIDs
	// kosong) → skip delete, cart tidak disentuh.
	if len(consumedCartIDs) > 0 {
		if err := tx.Where("user_id = ? AND id IN ?", userID, consumedCartIDs).Delete(&entity.EcomCartItem{}).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to clear cart"}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to commit"}
	}

	voucherCodeStr := ""
	if voucherCodePtr != nil {
		voucherCodeStr = *voucherCodePtr
	}
	return &dto.CheckoutCreateResponse{
		OrderID:         orderID,
		Subtotal:        subtotal,
		ShippingCost:    req.ShippingCost,
		VoucherCode:     voucherCodeStr,
		VoucherDiscount: voucherDiscount,
		Total:           total,
		SnapToken:       snapToken,
		SnapRedirectURL: snapRedirectURL,
		PaymentMode:     paymentMode,
		EcomStatus:      ecomStatus,
	}, nil
}

// HandleMidtransNotification — webhook dari Midtrans setiap payment status change.
// Update ecom_status + paid_at. Rollback stock kalau expired/cancel.
func (s *EcomCheckoutService) HandleMidtransNotification(notif midtrans.Notification) *dto.ApiError {
	var order entity.Order
	if err := s.DB.Where("id = ?", notif.OrderID).First(&order).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}
	if order.OrderSource != "ecom" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Not an ecom order"}
	}

	newStatus := ""
	shouldRestoreStock := false
	shouldMarkPaid := false

	switch notif.TransactionStatus {
	case "settlement", "capture":
		newStatus = "paid"
		shouldMarkPaid = true
	case "pending":
		newStatus = "pending_payment"
	case "expire":
		newStatus = "expired"
		shouldRestoreStock = true
	case "cancel", "deny":
		newStatus = "cancelled"
		shouldRestoreStock = true
	}

	if newStatus == "" {
		return nil // ignore unknown status
	}

	tx := s.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updates := map[string]interface{}{
		"ecom_status":       newStatus,
		"payment_reference": notif.TransactionID,
		"payment":           notif.PaymentType,
	}
	if shouldMarkPaid {
		now := time.Now()
		updates["payment_paid_at"] = now
		updates["status"] = "completed" // POS side status
	}
	if err := tx.Model(&entity.Order{}).Where("id = ?", notif.OrderID).Updates(updates).Error; err != nil {
		tx.Rollback()
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update order"}
	}

	if shouldRestoreStock {
		var items []entity.OrderItem
		tx.Where("order_id = ?", notif.OrderID).Find(&items)
		for _, it := range items {
			tx.Model(&entity.Product{}).Where("id = ?", it.ProductID).
				Update("stock_ecom", gorm.Expr("stock_ecom + ?", it.Quantity))
		}
	}

	if err := tx.Commit().Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to commit"}
	}

	s.Log.Info().
		Str("order_id", notif.OrderID).
		Str("new_status", newStatus).
		Str("txn_status", notif.TransactionStatus).
		Msg("Midtrans notification processed")

	// Kirim email konfirmasi pembayaran — best-effort di goroutine, tidak
	// block response webhook (Midtrans retry kalau webhook lambat > 5 detik).
	// Fetch order + user email fresh untuk hindari stale data.
	if shouldMarkPaid {
		go s.sendOrderPaidEmail(notif.OrderID)
	}

	return nil
}

// sendOrderPaidEmail — dipanggil setelah order transition ke paid.
// Fetch order + user + items, render HTML sederhana, kirim via Brevo.
// Kalau Brevo tidak configured → Email.Send stub log ke stdout, tetap OK.
func (s *EcomCheckoutService) sendOrderPaidEmail(orderID string) {
	var order entity.Order
	if err := s.DB.Preload("Items").Where("id = ?", orderID).First(&order).Error; err != nil {
		s.Log.Warn().Err(err).Str("order_id", orderID).Msg("email: order fetch failed")
		return
	}
	if order.EcomUserID == nil || *order.EcomUserID == "" {
		return
	}
	var user entity.User
	if err := s.DB.Where("id = ?", *order.EcomUserID).First(&user).Error; err != nil {
		return
	}
	if user.Email == "" {
		return
	}

	// Recipient info dari shipping snapshot.
	recipientName := user.FullName
	if order.ShippingAddressSnapshot != nil {
		var snap map[string]interface{}
		if err := json.Unmarshal([]byte(*order.ShippingAddressSnapshot), &snap); err == nil {
			if n, ok := snap["recipient_name"].(string); ok && n != "" {
				recipientName = n
			}
		}
	}

	shortID := orderID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	// Build items row HTML.
	itemsHTML := ""
	for _, it := range order.Items {
		itemsHTML += fmt.Sprintf(
			`<tr><td style="padding:6px 0">%s <span style="color:#999">×%d</span></td><td align="right" style="padding:6px 0">Rp %s</td></tr>`,
			it.Name, it.Quantity, formatRpForEmail(it.UnitPrice*float64(it.Quantity)))
	}
	trackingURL := s.AppURL
	if trackingURL == "" {
		trackingURL = "https://tbksanti.id"
	}
	trackingURL += "/shop/pesanan/" + orderID

	subject := "Pembayaran diterima — Order #" + shortID + " TBK Santi"
	html := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:24px">
<h2 style="color:#C4302B;margin:0 0 8px">Pembayaran Diterima</h2>
<p>Hai %s, terima kasih!</p>
<p>Pembayaran untuk order <b>#%s</b> sudah kami terima. Pesananmu segera diproses.</p>
<table style="width:100%%;border-collapse:collapse;font-size:14px;margin:12px 0">
%s
<tr><td colspan="2" style="border-top:1px solid #eee;padding-top:8px"></td></tr>
<tr><td>Subtotal</td><td align="right">Rp %s</td></tr>
<tr><td>Ongkir</td><td align="right">Rp %s</td></tr>
<tr style="font-weight:900;color:#C4302B"><td>Total</td><td align="right">Rp %s</td></tr>
</table>
<a href="%s" style="display:inline-block;padding:10px 20px;background:#C4302B;color:#fff;border-radius:8px;text-decoration:none;font-weight:bold;margin-top:8px">Lihat Detail Pesanan</a>
<p style="color:#999;font-size:12px;margin-top:24px">Toko Bahan Kue Santi · tbksanti.id</p>
</div>`,
		recipientName, shortID, itemsHTML,
		formatRpForEmail(order.Subtotal), formatRpForEmail(order.ShippingCost), formatRpForEmail(order.Total),
		trackingURL)

	text := fmt.Sprintf("Order #%s: pembayaran diterima. Total Rp %s. Lihat detail: %s",
		shortID, formatRpForEmail(order.Total), trackingURL)

	if err := s.Email.Send(user.Email, recipientName, subject, html, text); err != nil {
		s.Log.Warn().Err(err).Str("order_id", orderID).Str("email", user.Email).Msg("order paid email send failed")
	}
}

// formatRpForEmail — simple integer format dengan dot separator.
// Tidak pakai locale-aware library supaya dependency ringan.
func formatRpForEmail(n float64) string {
	i := int64(n)
	s := fmt.Sprintf("%d", i)
	// Insert dot tiap 3 digit dari kanan.
	if len(s) <= 3 {
		return s
	}
	out := ""
	for j, c := range []rune(s) {
		if j > 0 && (len(s)-j)%3 == 0 {
			out += "."
		}
		out += string(c)
	}
	return out
}
