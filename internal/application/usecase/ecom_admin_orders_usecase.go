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
	"github.com/faizalramadhan/pos-be/internal/infrastructure/biteship"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// extractSnapshotField — decode 1 key dari JSON snapshot address. Kalau parse
// fail atau key tidak ada, return empty string (safe untuk display).
func extractSnapshotField(raw, key string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// EcomAdminOrdersService — admin panel untuk kelola pesanan ecom.
// Bu Santi 27 Jul 2026 — Sprint 1 blocker: tanpa panel ini, order masuk
// tidak bisa di-fulfill (no view list, no mark shipped, no input resi).
type EcomAdminOrdersService struct {
	DB       *gorm.DB
	Log      *zerolog.Logger
	Biteship *biteship.Client
	Cfg      *config.Config
}

func NewEcomAdminOrdersService(ctx context.Context, db *gorm.DB) *EcomAdminOrdersService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomAdminOrdersService{
		DB:       db,
		Log:      logger,
		Biteship: NewBiteshipClient(cfg),
		Cfg:      cfg,
	}
}

// AdminList — semua ecom orders, filter by status/search, cursor pagination.
// Return list untuk table view di admin panel Pesanan Online.
func (s *EcomAdminOrdersService) List(status, search, cursor string, limit int) (*dto.EcomAdminOrderListResponse, *dto.ApiError) {
	q := s.DB.Model(&entity.Order{}).
		Where("order_source = ? AND deleted_at IS NULL", "ecom")

	if status != "" && status != "all" {
		q = q.Where("ecom_status = ?", status)
	}
	if search != "" {
		like := "%" + strings.TrimSpace(search) + "%"
		// Match order id (short prefix), customer name/phone (dari shipping snapshot JSON).
		q = q.Where("id LIKE ? OR JSON_EXTRACT(shipping_address_snapshot, '$.recipient_name') LIKE ? OR JSON_EXTRACT(shipping_address_snapshot, '$.recipient_phone') LIKE ?",
			like, like, like)
	}
	if cursor != "" {
		if t, err := time.Parse(time.RFC3339, cursor); err == nil {
			q = q.Where("created_at < ?", t)
		}
	}

	var orders []entity.Order
	if err := q.Preload("Items").Order("created_at DESC").Limit(limit).Find(&orders).Error; err != nil {
		s.Log.Error().Err(err).Msg("admin orders list failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch orders"}
	}

	items := make([]dto.EcomAdminOrderListItem, 0, len(orders))
	for _, o := range orders {
		ecomStatus := ""
		if o.EcomStatus != nil {
			ecomStatus = *o.EcomStatus
		}
		recipient := ""
		if o.ShippingAddressSnapshot != nil {
			recipient = extractSnapshotField(*o.ShippingAddressSnapshot, "recipient_name")
		}
		items = append(items, dto.EcomAdminOrderListItem{
			ID:            o.ID,
			Total:         o.Total,
			ShippingCost:  o.ShippingCost,
			EcomStatus:    ecomStatus,
			ItemCount:     len(o.Items),
			Recipient:     recipient,
			Courier:       ptrString(o.ShippingCourier),
			AWB:           ptrString(o.ShippingAWB),
			CreatedAt:     o.CreatedAt.Format(time.RFC3339),
			PaymentMethod: o.Payment,
		})
	}

	nextCursor := ""
	if len(orders) == limit && limit > 0 {
		nextCursor = orders[len(orders)-1].CreatedAt.Format(time.RFC3339)
	}

	// Count per status untuk badge di FE tabs.
	var counts []struct {
		EcomStatus string
		Cnt        int
	}
	s.DB.Model(&entity.Order{}).
		Select("ecom_status, COUNT(*) as cnt").
		Where("order_source = 'ecom' AND deleted_at IS NULL").
		Group("ecom_status").Scan(&counts)
	countMap := map[string]int{}
	for _, c := range counts {
		countMap[c.EcomStatus] = c.Cnt
	}

	return &dto.EcomAdminOrderListResponse{
		Items:        items,
		NextCursor:   nextCursor,
		CountsByStatus: countMap,
	}, nil
}

// GetDetail — reuse customer detail structure (sudah lengkap), tapi
// tanpa filter user_id (admin lihat semua).
func (s *EcomAdminOrdersService) GetDetail(orderID string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	// Delegate ke customer GetDetail dengan wildcard user (query builder pattern
	// tidak clean — direct fetch di sini lebih explicit).
	var order entity.Order
	if err := s.DB.Preload("Items").Preload("Member").
		Where("id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}
	// Build response mirror customer detail — kalau nanti admin butuh field
	// extra (mis. IP, device fingerprint) tinggal extend di sini.
	return buildEcomOrderDetail(&order), nil
}

// UpdateStatus — admin transition state. Validate transisi allowed supaya
// admin tidak accidentally "mark shipped" order yang belum bayar.
func (s *EcomAdminOrdersService) UpdateStatus(orderID, newStatus, changedBy string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	var order entity.Order
	if err := s.DB.Where("id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID).First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}

	current := ""
	if order.EcomStatus != nil {
		current = *order.EcomStatus
	}

	if !isValidTransition(current, newStatus) {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Tidak bisa ubah status " + current + " → " + newStatus,
		}
	}

	tx := s.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updates := map[string]interface{}{"ecom_status": newStatus}

	// Snapshot waktu tandai sampai — dipakai cron auto-complete 7 hari + FE
	// display "Sampai N hari lalu, konfirmasi ya!" reminder.
	if newStatus == "delivered" {
		updates["ecom_delivered_at"] = time.Now()
	}

	// Cancel → restore stock_ecom + mark POS-side status.
	if newStatus == "cancelled" {
		var items []entity.OrderItem
		tx.Where("order_id = ?", orderID).Find(&items)
		for _, it := range items {
			if it.ProductID == "" {
				continue
			}
			if err := tx.Model(&entity.Product{}).Where("id = ?", it.ProductID).
				Update("stock_ecom", gorm.Expr("stock_ecom + ?", it.Quantity)).Error; err != nil {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to restore stock"}
			}
		}
		updates["status"] = "cancelled"
	}

	// Completed → POS side juga completed (kalau belum). Sudah completed dari
	// paid transition, jadi safe untuk re-set.
	if newStatus == "completed" {
		updates["status"] = "completed"
	}

	if err := tx.Model(&entity.Order{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update status"}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to commit"}
	}

	s.Log.Info().Str("order_id", orderID).Str("from", current).Str("to", newStatus).Str("by", changedBy).Msg("ecom order status changed")

	return s.GetDetail(orderID)
}

// SetShipping — input AWB (nomor resi) + optional update courier/service.
// Auto-transition ke 'shipped' kalau saat ini 'processing' atau 'paid'.
func (s *EcomAdminOrdersService) SetShipping(orderID, awb, courier, service, changedBy string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	if strings.TrimSpace(awb) == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Nomor resi wajib"}
	}
	var order entity.Order
	if err := s.DB.Where("id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID).First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}
	current := ""
	if order.EcomStatus != nil {
		current = *order.EcomStatus
	}
	if current != "paid" && current != "processing" && current != "shipped" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Set resi hanya untuk order paid/processing/shipped (current: " + current + ")"}
	}

	updates := map[string]interface{}{
		"shipping_awb": awb,
		"ecom_status":  "shipped",
	}
	if courier != "" {
		updates["shipping_courier"] = courier
	}
	if service != "" {
		updates["shipping_service"] = service
	}

	if err := s.DB.Model(&entity.Order{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save resi"}
	}

	s.Log.Info().Str("order_id", orderID).Str("awb", awb).Str("by", changedBy).Msg("ecom order resi set + shipped")
	return s.GetDetail(orderID)
}

// isValidTransition — enforce state machine. Kalau invalid, tolak — cegah
// admin accidentally klik tombol yang tidak masuk akal.
//
// Flow customer-confirmation (28 Jul 2026, mirip Tokopedia/Shopee):
//   shipped   → delivered  (kurir tandai sampai via Biteship webhook, atau
//                           admin manual)
//   delivered → completed  (customer klik "Konfirmasi Diterima")
//   shipped   → completed  (admin force complete kalau customer complain
//                           sudah terima tapi lupa konfirmasi — edge case)
//   delivered → cancelled  (kalau ada dispute / komplain valid → refund flow)
func isValidTransition(from, to string) bool {
	allowed := map[string][]string{
		"pending_payment": {"paid", "cancelled", "expired"},
		"paid":            {"processing", "shipped", "cancelled"},
		"processing":      {"shipped", "cancelled"},
		"shipped":         {"delivered", "completed", "cancelled"},
		"delivered":       {"completed", "cancelled"},
		// Terminal states — no transition out.
		"completed": {},
		"cancelled": {},
		"expired":   {},
	}
	for _, t := range allowed[from] {
		if t == to {
			return true
		}
	}
	return false
}

// buildEcomOrderDetail — build DTO shared antara admin + customer view.
// Ini di-extract dari EcomOrdersService.GetDetail supaya bisa di-reuse.
func buildEcomOrderDetail(order *entity.Order) *dto.CustomerOrderDetail {
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
	if order.ShippingAddressSnapshot != nil {
		snap := *order.ShippingAddressSnapshot
		detail.Shipping.Address.Label = extractSnapshotField(snap, "label")
		detail.Shipping.Address.RecipientName = extractSnapshotField(snap, "recipient_name")
		detail.Shipping.Address.RecipientPhone = extractSnapshotField(snap, "recipient_phone")
		detail.Shipping.Address.StreetAddress = extractSnapshotField(snap, "street_address")
		detail.Shipping.Address.Subdistrict = extractSnapshotField(snap, "subdistrict")
		detail.Shipping.Address.District = extractSnapshotField(snap, "district")
		detail.Shipping.Address.City = extractSnapshotField(snap, "city")
		detail.Shipping.Address.Province = extractSnapshotField(snap, "province")
		detail.Shipping.Address.Zipcode = extractSnapshotField(snap, "zipcode")
		detail.Shipping.Address.Notes = extractSnapshotField(snap, "notes")
	}
	// Payment — PG DOKU (28 Jul 2026). Admin cukup lihat mode + channel +
	// reference; tombol "Bayar" hanya di sisi customer.
	if order.PaymentURL != nil && *order.PaymentURL != "" && !isStubToken(*order.PaymentURL) {
		detail.Payment.Mode = "pg"
		// PaymentURL sengaja tidak di-expose ke admin — supaya admin tidak
		// accidentally klik + tercatat sebagai "customer buka" di analytics PG.
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
	return detail
}

func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ─── Biteship Order API (Sprint 3) ───────────────────────────────────

// CreateBiteshipShipment — trigger Biteship Order API untuk generate resi
// auto. Prereq:
//   - Order status = 'paid' atau 'processing' (customer sudah bayar, siap kirim)
//   - Belum ada biteship_order_id (cegah double create → double charge Biteship)
//   - Biteship client punya shipper info lengkap (config)
//
// Setelah sukses:
//   - Simpan biteship_order_id + waybill (kalau langsung ada — sering baru
//     terisi via webhook setelah pickup)
//   - Status TIDAK auto-jump ke 'shipped' — tunggu waybill dulu (webhook fires
//     saat kurir konfirmasi pickup). Admin bisa mark shipped manual kalau
//     tidak sabar.
func (s *EcomAdminOrdersService) CreateBiteshipShipment(orderID, changedBy string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	if !s.Biteship.CanCreateOrder() {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "Biteship shipper info belum di-set (BITESHIP_ORIGIN_NAME/PHONE/ADDRESS). Cek .env di server.",
		}
	}

	var order entity.Order
	if err := s.DB.Preload("Items").Where("id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID).First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}
	current := ""
	if order.EcomStatus != nil {
		current = *order.EcomStatus
	}
	if current != "paid" && current != "processing" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Buat resi Biteship hanya untuk order paid/processing (current: " + current + ")"}
	}
	if order.BiteshipOrderID != nil && *order.BiteshipOrderID != "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order sudah punya resi Biteship (id: " + *order.BiteshipOrderID + ")"}
	}

	// Destination info dari shipping_address_snapshot.
	if order.ShippingAddressSnapshot == nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order tidak punya alamat pengiriman"}
	}
	var snap map[string]interface{}
	if err := json.Unmarshal([]byte(*order.ShippingAddressSnapshot), &snap); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Address snapshot corrupt"}
	}
	get := func(k string) string {
		if v, ok := snap[k].(string); ok {
			return v
		}
		return ""
	}
	destName := get("recipient_name")
	destPhone := get("recipient_phone")
	destPostal := get("zipcode")
	destAddress := strings.Join([]string{
		get("street_address"),
		get("subdistrict"),
		get("district"),
		get("city"),
		get("province"),
	}, ", ")
	destNote := get("notes")

	// Courier code — order.ShippingCourier disimpan sebagai display name
	// (mis. "JNE"), tapi Biteship API accept lowercase code ("jne"). Simple
	// downcase — kalau nanti ada courier dengan multi-word, tambah mapper.
	courierCode := ""
	if order.ShippingCourier != nil {
		courierCode = strings.ToLower(strings.ReplaceAll(*order.ShippingCourier, " ", ""))
		// Common mapping — Biteship pakai j&t → jnt, sicepat OK, dst.
		courierCode = strings.ReplaceAll(courierCode, "&", "")
	}
	if courierCode == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order tidak punya kurir terpilih"}
	}
	courierService := "reg"
	if order.ShippingService != nil && *order.ShippingService != "" {
		courierService = strings.ToLower(*order.ShippingService)
	}

	// Items untuk insurance calc.
	items := make([]biteship.Item, 0, len(order.Items))
	for _, it := range order.Items {
		if it.ProductID == "" {
			continue
		}
		// Weight per unit tidak di-snapshot di order_items — fallback 200g
		// (Biteship butuh angka, TIDAK boleh 0). Kalau nanti weight critical,
		// snapshot ke order_items.
		items = append(items, biteship.Item{
			Name:     it.Name,
			Value:    int(it.UnitPrice),
			Weight:   200,
			Quantity: it.Quantity,
		})
	}

	// Total value untuk insurance.
	insurance := int(order.Subtotal)
	if insurance < 10000 {
		insurance = 0 // insurance minimum di kebanyakan kurir; skip kalau kecil
	}

	req := biteship.CreateOrderRequest{
		DestinationName:    destName,
		DestinationPhone:   destPhone,
		DestinationAddress: destAddress,
		DestinationPostal:  destPostal,
		DestinationNote:    destNote,
		CourierCode:        courierCode,
		CourierService:     courierService,
		Items:              items,
		OrderNote:          "Order #" + orderID[:8],
		CourierInsurance:   insurance,
		ReferenceID:        orderID, // supaya webhook bisa lookup order kita
	}

	resp, err := s.Biteship.CreateOrder(req)
	if err != nil {
		s.Log.Error().Err(err).Str("order_id", orderID).Msg("biteship create order failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadGateway, Message: "Gagal buat order Biteship: " + err.Error()}
	}

	// Simpan biteship_order_id (+ waybill kalau langsung ada + auto-transition
	// ke 'processing' untuk tanda admin sudah trigger create).
	updates := map[string]interface{}{
		"biteship_order_id": resp.OrderID,
		"ecom_status":       "processing",
	}
	if resp.Waybill != "" {
		updates["shipping_awb"] = resp.Waybill
	}
	if err := s.DB.Model(&entity.Order{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
		s.Log.Error().Err(err).Str("order_id", orderID).Msg("save biteship_order_id failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Order Biteship dibuat tapi gagal simpan id. Cek log server."}
	}

	s.Log.Info().
		Str("order_id", orderID).
		Str("biteship_id", resp.OrderID).
		Str("status", resp.Status).
		Str("by", changedBy).
		Msg("biteship shipment created")

	return s.GetDetail(orderID)
}

// HandleBiteshipWebhook — dipanggil dari handler webhook publik.
// Biteship kirim event `order.status` + `order.waybill_id` — kita update
// mapping ke internal ecom_status kita.
//
// Payload signature verified di handler layer (raw body).
func (s *EcomAdminOrdersService) HandleBiteshipWebhook(payload BiteshipWebhookPayload) *dto.ApiError {
	// Cari order via biteship_order_id atau reference_id (kita set = orderID).
	var order entity.Order
	q := s.DB.Where("order_source = 'ecom' AND deleted_at IS NULL")
	if payload.OrderID != "" {
		q = q.Where("biteship_order_id = ?", payload.OrderID)
	} else if payload.ReferenceID != "" {
		q = q.Where("id = ?", payload.ReferenceID)
	} else {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Payload tidak punya order_id atau reference_id"}
	}
	if err := q.First(&order).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}

	updates := map[string]interface{}{}
	if payload.WaybillID != "" {
		updates["shipping_awb"] = payload.WaybillID
	}

	// Map Biteship status → ecom_status kita.
	// Biteship status: allocated, picking_up, picked, dropping_off, delivered,
	// returned, cancelled, on_hold, disposed.
	if payload.Status != "" {
		mapped := mapBiteshipStatus(payload.Status, ptrString(order.EcomStatus))
		if mapped != "" {
			updates["ecom_status"] = mapped
			// Kurir tandai sampai — stamp delivered_at supaya cron auto-complete
			// dan FE reminder tau umur-nya.
			if mapped == "delivered" {
				updates["ecom_delivered_at"] = time.Now()
			}
		}
	}

	if len(updates) == 0 {
		return nil // nothing to do
	}
	if err := s.DB.Model(&entity.Order{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
		s.Log.Error().Err(err).Str("order_id", order.ID).Msg("biteship webhook update failed")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update order"}
	}
	s.Log.Info().
		Str("order_id", order.ID).
		Str("biteship_id", payload.OrderID).
		Str("biteship_status", payload.Status).
		Str("waybill", payload.WaybillID).
		Msg("biteship webhook processed")
	return nil
}

// BiteshipWebhookPayload — subset field yang kita butuh dari webhook.
// Biteship pakai flat structure di top-level plus optional nested `courier`.
type BiteshipWebhookPayload struct {
	Event       string `json:"event"`        // "order.status" | "order.waybill_id"
	OrderID     string `json:"order_id"`     // Biteship order id
	ReferenceID string `json:"reference_id"` // = orderID kita (yang di-set saat CreateOrder)
	Status      string `json:"status"`
	WaybillID   string `json:"waybill_id"`
	Courier     struct {
		Company   string `json:"company"`
		WaybillID string `json:"waybill_id"`
	} `json:"courier"`
}

// Normalize — pindahkan nested courier.waybill_id ke top-level kalau kosong.
func (p *BiteshipWebhookPayload) Normalize() {
	if p.WaybillID == "" && p.Courier.WaybillID != "" {
		p.WaybillID = p.Courier.WaybillID
	}
}

// mapBiteshipStatus — konvert Biteship internal status ke ecom_status kita.
// current = status saat ini, dipakai untuk cegah backwards transition
// (mis. Biteship kirim 'allocated' setelah kita sudah 'shipped', jangan
// downgrade).
func mapBiteshipStatus(biteshipStatus, current string) string {
	// Terminal — tidak boleh berubah.
	if current == "completed" || current == "cancelled" {
		return ""
	}
	switch biteshipStatus {
	case "allocated", "picking_up", "on_hold":
		// Kurir sudah accept, sedang proses pickup — di kita masih "processing".
		if current == "paid" {
			return "processing"
		}
		return ""
	case "picked", "dropping_off":
		return "shipped"
	case "delivered":
		// Kurir bilang sampai — TIDAK langsung completed. Marketplace pattern
		// (Tokopedia/Shopee/Blibli): customer harus konfirmasi manual, sebab
		// kurir kadang salah tag (dititip, salah alamat, dsb). Fallback:
		// cron auto-complete 7 hari kalau customer lupa.
		return "delivered"
	case "cancelled", "returned", "disposed":
		return "cancelled"
	}
	return ""
}
