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
	"github.com/faizalramadhan/pos-be/internal/infrastructure/midtrans"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomCheckoutService struct {
	DB       *gorm.DB
	Log      *zerolog.Logger
	Midtrans *midtrans.Client
	AppURL   string
}

func NewEcomCheckoutService(ctx context.Context, db *gorm.DB) *EcomCheckoutService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomCheckoutService{
		DB:       db,
		Log:      logger,
		Midtrans: midtrans.NewClient(cfg.MidtransServerKey, cfg.MidtransIsProd),
		AppURL:   cfg.AppURL,
	}
}

// CreateOrder — end-to-end checkout: validate cart + address, create Order,
// decrement stock_ecom (reserve), clear cart, request Snap token.
// Kalau Midtrans belum configured, mode "manual" (customer transfer bank
// manual, Bu Santi verify).
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

	var cart []entity.EcomCartItem
	if err := tx.Preload("Product").Where("user_id = ?", userID).Find(&cart).Error; err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
	}
	if len(cart) == 0 {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Keranjang kosong"}
	}

	// Validate + compute subtotal + reserve stock.
	subtotal := 0.0
	orderID := uuid.New().String()
	orderItems := make([]entity.OrderItem, 0, len(cart))
	for _, c := range cart {
		p := c.Product
		if p == nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Produk cart tidak valid"}
		}
		if !p.EcomIsAvailable {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Produk %s tidak tersedia", p.NameID)}
		}
		if c.Quantity > p.StockEcom {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Stok %s tidak cukup (tersisa %d)", p.NameID, p.StockEcom)}
		}
		price := p.SellingPrice
		if p.EcomPrice != nil {
			price = *p.EcomPrice
		}
		lineTotal := price * float64(c.Quantity)
		subtotal += lineTotal

		orderItems = append(orderItems, entity.OrderItem{
			ID:            uuid.New().String(),
			OrderID:       orderID,
			ProductID:     p.ID,
			Name:          p.NameID,
			Quantity:      c.Quantity,
			UnitType:      "individual",
			UnitPrice:     price,
			PurchasePrice: p.PurchasePrice,
		})

		// Reserve stock — decrement stock_ecom + stock (pos). Karena Bu Santi
		// pilih strict separation, decrement dari stock_ecom saja. Stok POS
		// tetap = tidak affected.
		if err := tx.Model(&entity.Product{}).Where("id = ?", p.ID).
			Update("stock_ecom", gorm.Expr("stock_ecom - ?", c.Quantity)).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to reserve stock"}
		}
	}

	total := subtotal + req.ShippingCost

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

	// Clear cart items user (order sudah di-buat).
	if err := tx.Where("user_id = ?", userID).Delete(&entity.EcomCartItem{}).Error; err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to clear cart"}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to commit"}
	}

	return &dto.CheckoutCreateResponse{
		OrderID:         orderID,
		Subtotal:        subtotal,
		ShippingCost:    req.ShippingCost,
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

	return nil
}
