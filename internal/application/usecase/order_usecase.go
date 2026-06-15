package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/whatsapp"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Threshold untuk transaksi "besar" yang trigger notif WA ke admin/owner.
// Bu Santi minta hanya kirim notif kalau total ≥ Rp 500.000 (transaksi
// besar saja, supaya tidak spam quota WA + nomor toko tidak ke-flag karena
// volume tinggi). Kalau di masa depan mau dynamic, pindah ke settings table.
const largeTransactionThreshold = 500_000.0

type OrderService struct {
	Log          *zerolog.Logger
	DB           *gorm.DB
	Repo         *repository.OrderRepository
	ProductRepo  *repository.ProductRepository
	BatchRepo    *repository.StockBatchRepository
	SettingsRepo *repository.SettingsRepository
	AuthRepo     *repository.AuthRepository
	MemberRepo   *repository.MemberRepository
	PointMoveRepo *repository.MemberPointMovementRepository
	WA           *whatsapp.Service
	Configs      *config.Config
}

func NewOrderService(ctx context.Context, db *gorm.DB) *OrderService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	wa, _ := ctx.Value(enum.WhatsAppCtxKey).(*whatsapp.Service)
	cfg, _ := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &OrderService{
		Log:           logger,
		DB:            db,
		Repo:          repository.NewOrderRepository(ctx, db),
		ProductRepo:   repository.NewProductRepository(ctx, db),
		BatchRepo:     repository.NewStockBatchRepository(ctx, db),
		SettingsRepo:  repository.NewSettingsRepository(ctx, db),
		AuthRepo:      repository.NewAuthRepository(ctx, db),
		MemberRepo:    repository.NewMemberRepository(ctx, db),
		PointMoveRepo: repository.NewMemberPointMovementRepository(ctx, db),
		WA:            wa,
		Configs:       cfg,
	}
}

// Earn 1.000 poin per kelipatan Rp 100.000 di cash actual. STRICT —
// hanya kelipatan tepat dapat poin, 150rb/199rb tidak.
const (
	pointsEarnPerUnit    = 1_000
	pointsEarnUnitAmount = 100_000.0
)

// calculateEarnedPoints — strict exact-multiple rule. 100rb=1000, 150rb=0,
// 200rb=2000, 250rb=0, 300rb=3000. Cash actual = total cash setelah
// dikurangi item yang ditebus pakai poin.
func calculateEarnedPoints(cashActual float64) int {
	if cashActual < pointsEarnUnitAmount {
		return 0
	}
	// Bandingkan integer untuk hindari float-comparison rounding.
	cents := int64(cashActual * 100)
	unitCents := int64(pointsEarnUnitAmount * 100)
	if cents%unitCents != 0 {
		return 0
	}
	return int(cents/unitCents) * pointsEarnPerUnit
}

func (s *OrderService) GetAll(status string, page, limit int) ([]dto.OrderResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	orders, total, err := s.Repo.FindAll(status, limit, offset)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch orders")
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch orders"}
	}

	var result []dto.OrderResponse
	for _, o := range orders {
		result = append(result, s.toResponse(&o))
	}
	return result, total, nil
}

func (s *OrderService) GetByID(id string) (*dto.OrderResponse, *dto.ApiError) {
	order, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}
	resp := s.toResponse(order)
	return &resp, nil
}

func (s *OrderService) Create(req dto.CreateOrderRequest, userID string) (*dto.OrderResponse, *dto.ApiError) {
	// Hitung poin yang akan ditebus per cart: sum harga item flagged
	// redeem_with_points. Validate member ada & saldo cukup di awal,
	// supaya tidak mid-checkout rollback besar.
	var pointsToRedeem int
	var cashSubtotal float64
	for _, item := range req.Items {
		lineTotal := item.UnitPrice * float64(item.Quantity)
		if item.RedeemWithPoints {
			pointsToRedeem += int(lineTotal)
		} else {
			cashSubtotal += lineTotal
		}
	}
	if pointsToRedeem > 0 {
		if req.MemberID == nil || *req.MemberID == "" {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Tebus barang dengan poin perlu pilih member dulu"}
		}
		member, err := s.MemberRepo.FindByID(*req.MemberID)
		if err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Member tidak ditemukan"}
		}
		if member.Points < pointsToRedeem {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Saldo poin tidak cukup: butuh %d, tersedia %d", pointsToRedeem, member.Points)}
		}
	}

	// Split-payment validation: if payments are provided, their sum must
	// cover the cash portion (after subtracting redeemed items). When no
	// item is redeemed, cashSubtotal == req.Total.
	payments := req.Payments
	if len(payments) == 0 {
		payments = []dto.CreateOrderPaymentRequest{{Method: req.Payment, Amount: cashSubtotal}}
	}
	var paidSum float64
	for _, p := range payments {
		paidSum += p.Amount
	}
	if paidSum+0.001 < cashSubtotal { // tolerate fp rounding
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Kurang bayar: cash perlu Rp %.0f, diterima Rp %.0f", cashSubtotal, paidSum)}
	}

	tx := s.DB.Begin()

	order := &entity.Order{
		ID:                 uuid.New().String(),
		Subtotal:           req.Subtotal,
		PPNRate:            req.PPNRate,
		PPN:                req.PPN,
		Total:              req.Total,
		Payment:            primaryPaymentMethod(payments),
		Status:             "completed",
		Customer:           req.Customer,
		CustomerPhone:      req.CustomerPhone,
		MemberID:           req.MemberID,
		PaymentProof:       req.PaymentProof,
		OrderDiscountType:  req.OrderDiscountType,
		OrderDiscountValue: req.OrderDiscountValue,
		OrderDiscount:      req.OrderDiscount,
		CreatedBy:          userID,
	}
	for _, p := range payments {
		order.Payments = append(order.Payments, entity.OrderPayment{
			ID:      uuid.New().String(),
			OrderID: order.ID,
			Method:  p.Method,
			Amount:  p.Amount,
		})
	}

	for _, item := range req.Items {
		orderItem := entity.OrderItem{
			ID:                 uuid.New().String(),
			OrderID:            order.ID,
			ProductID:          item.ProductID,
			Name:               item.Name,
			Quantity:           item.Quantity,
			UnitType:           item.UnitType,
			UnitPrice:          item.UnitPrice,
			PurchasePrice:      item.PurchasePrice,
			RegularPrice:       item.RegularPrice,
			DiscountType:       item.DiscountType,
			DiscountValue:      item.DiscountValue,
			DiscountAmount:     item.DiscountAmount,
			RedeemedWithPoints: item.RedeemWithPoints,
		}
		if orderItem.UnitType == "" {
			orderItem.UnitType = "individual"
		}

		// Deduct stock
		product, err := s.ProductRepo.FindByID(item.ProductID)
		if err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Product not found: " + item.ProductID}
		}

		// Always capture regular_price snapshot at sale time:
		// for member sales we need it to compute savings; for non-member we
		// store selling_price so the historical record is self-contained.
		if orderItem.RegularPrice == nil {
			rp := product.SellingPrice
			if orderItem.UnitType == "box" && product.QtyPerBox > 0 {
				rp = product.SellingPrice * float64(product.QtyPerBox)
			}
			orderItem.RegularPrice = &rp
		}

		// Snapshot purchase price at sale time for accurate profit reporting
		// (owner dashboard shows GROSS vs NET = revenue - COGS). If the
		// product purchase price changes later, historical profit stays correct.
		purchase := product.PurchasePrice
		if orderItem.UnitType == "box" && product.QtyPerBox > 0 {
			purchase = product.PurchasePrice * float64(product.QtyPerBox)
		}
		orderItem.PurchasePrice = purchase

		order.Items = append(order.Items, orderItem)

		stockDelta := item.Quantity
		if item.UnitType == "box" && product.QtyPerBox > 0 {
			stockDelta = item.Quantity * product.QtyPerBox
		}

		if product.Stock < stockDelta {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Insufficient stock for: " + product.Name}
		}

		if err := tx.Model(&entity.Product{}).Where("id = ?", item.ProductID).
			Update("stock", gorm.Expr("stock - ?", stockDelta)).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to deduct stock"}
		}

		// Consume FIFO batches
		s.consumeFIFO(tx, item.ProductID, stockDelta)

		// Audit trail: insert stock_movements record type='out' so reports
		// can reconstruct stock-out history. Tanpa ini, "Total Barang Keluar"
		// dan Recent Movements jadi kosong.
		if err := tx.Create(&entity.StockMovement{
			ID:         uuid.New().String(),
			ProductID:  item.ProductID,
			Type:       "out",
			Quantity:   stockDelta,
			UnitType:   item.UnitType,
			UnitPrice:  item.UnitPrice,
			Reason:     "sale",
			Note:       "Sale (Order #" + order.ID + ")",
			CreatedBy:  userID,
			CreatedAt:  time.Now(),
		}).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to record stock movement"}
		}
	}

	// Override order.Total dengan cash actual (tidak include item yang
	// ditebus pakai poin). Cara ini bikin Reports revenue match dengan
	// cash di laci, dan order_items tetap simpan unit_price asli + flag
	// redeem untuk audit "siapa beli apa kapan".
	order.Total = cashSubtotal

	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		s.Log.Error().Err(err).Msg("Failed to create order")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create order"}
	}

	// Member points: redeem dulu (kurangi saldo + log), lalu earn (tambah
	// saldo + log). Earn dihitung dari cashSubtotal — strict kelipatan 100k.
	// Loop in/out poin tidak ada karena item yang ditebus tidak masuk
	// cashSubtotal (sudah dipisah di awal).
	pointsEarned := calculateEarnedPoints(cashSubtotal)
	if req.MemberID != nil && *req.MemberID != "" && (pointsToRedeem > 0 || pointsEarned > 0) {
		if apiErr := s.applyPointsChange(tx, *req.MemberID, order.ID, userID, pointsToRedeem, pointsEarned); apiErr != nil {
			tx.Rollback()
			return nil, apiErr
		}
	}

	tx.Commit()

	created, _ := s.Repo.FindByID(order.ID)
	if created != nil {
		order = created
	}

	// WA struk ke customer: TIDAK auto-send lagi. Kasir kirim manual via
	// tombol "Kirim WA" di modal sukses POS — request owner setelah customer
	// kadang bingung dapat fisik + WA bersamaan. Manual send tetap pakai
	// endpoint ResendReceiptWA (POST /orders/:id/send-wa).
	//
	// Notifikasi admin tetap auto: HANYA untuk transaksi besar (≥ Rp 500.000)
	// supaya quota WA hemat + nomor toko tidak ke-flag karena volume tinggi.
	if order.Total >= largeTransactionThreshold {
		go s.sendLargeTransactionNotificationWA(order, userID)
	}

	resp := s.toResponse(order)
	return &resp, nil
}

// applyPointsChange does the member.points mutation + audit-trail inserts
// inside the supplied tx. Caller rolls back on error. Redeem first (so
// negative balance impossible), then earn. Either may be 0.
func (s *OrderService) applyPointsChange(tx *gorm.DB, memberID, orderID, userID string, redeem, earn int) *dto.ApiError {
	if redeem < 0 || earn < 0 {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Internal: negative points operation"}
	}
	// SELECT FOR UPDATE: lock member row for duration of tx supaya dua
	// concurrent checkout untuk member yang sama tidak race ke balance
	// negatif. MySQL InnoDB default REPEATABLE READ + row lock.
	var member entity.Member
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", memberID).First(&member).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Member tidak ditemukan saat update poin"}
	}
	balance := member.Points
	if redeem > 0 {
		if balance < redeem {
			return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Saldo poin tidak cukup (race)"}
		}
		balance -= redeem
		ord := orderID
		uid := userID
		if err := tx.Create(&entity.MemberPointMovement{
			ID:           uuid.New().String(),
			MemberID:     memberID,
			OrderID:      &ord,
			Type:         "redeem-item",
			Points:       -redeem,
			BalanceAfter: balance,
			Note:         "Tebus barang di order",
			CreatedBy:    &uid,
			CreatedAt:    time.Now(),
		}).Error; err != nil {
			return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to log redeem"}
		}
	}
	if earn > 0 {
		balance += earn
		ord := orderID
		uid := userID
		if err := tx.Create(&entity.MemberPointMovement{
			ID:           uuid.New().String(),
			MemberID:     memberID,
			OrderID:      &ord,
			Type:         "earn",
			Points:       earn,
			BalanceAfter: balance,
			Note:         "Belanja kelipatan Rp 100.000",
			CreatedBy:    &uid,
			CreatedAt:    time.Now(),
		}).Error; err != nil {
			return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to log earn"}
		}
	}
	if err := tx.Model(&entity.Member{}).Where("id = ?", memberID).Update("points", balance).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update member points"}
	}
	return nil
}

// ResendReceiptWA resends the WhatsApp receipt for an existing order.
// Runs synchronously so the frontend can surface errors back to the user.
func (s *OrderService) ResendReceiptWA(orderID, userID string) *dto.ApiError {
	if s.WA == nil || !s.WA.Enabled() {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "WhatsApp receipt is disabled"}
	}
	order, err := s.Repo.FindByID(orderID)
	if err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}
	phone := ""
	if order.Member != nil && order.Member.Phone != "" {
		phone = order.Member.Phone
	} else if order.CustomerPhone != "" {
		phone = order.CustomerPhone
	}
	if phone == "" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order has no phone number to send to"}
	}

	storeName := "Toko Bahan Kue Santi"
	storeAddress, storePhone := "", ""
	if settings, sErr := s.SettingsRepo.Get(); sErr == nil && settings != nil {
		if settings.StoreName != "" {
			storeName = settings.StoreName
		}
		storeAddress = settings.StoreAddress
		storePhone = settings.StorePhone
	}
	cashierName := ""
	if u, uErr := s.AuthRepo.FindByID(userID); uErr == nil && u != nil {
		cashierName = u.FullName
	}
	text := whatsapp.FormatReceipt(order, storeName, storeAddress, storePhone, cashierName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if sendErr := s.WA.SendText(ctx, phone, text); sendErr != nil {
		s.Log.Warn().Err(sendErr).Str("order_id", order.ID).Msg("WA resend failed")
		return &dto.ApiError{StatusCode: fiber.ErrBadGateway, Message: "Failed to send WhatsApp receipt: " + sendErr.Error()}
	}
	return nil
}

// sendLargeTransactionNotificationWA notifies admin/superadmin owners (with
// phone on file) of a transaction yang melewati threshold besar
// (largeTransactionThreshold). Hanya untuk transaksi penting — supaya quota
// WA hemat + cegah ban risk dari volume notif tinggi. Bu Santi confirm:
// "Iya lebih diset notifikasi dikirim pd transaksi >=500,000".
//
// Format pesan kasih emphasis "TRANSAKSI BESAR" supaya owner langsung tahu
// ini bukan notif rutin.
func (s *OrderService) sendLargeTransactionNotificationWA(order *entity.Order, userID string) {
	if s.WA == nil || !s.WA.Enabled() {
		return
	}
	admins, err := s.AuthRepo.FindAdmins()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to load admin list for large-tx WA")
		return
	}
	if len(admins) == 0 {
		return
	}

	cashierName := "-"
	if u, err := s.AuthRepo.FindByID(userID); err == nil && u != nil {
		cashierName = u.FullName
	}

	text := formatLargeTransactionNotification(order, cashierName)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, a := range admins {
		if err := s.WA.SendText(ctx, a.PhoneNumber, text); err != nil {
			s.Log.Warn().Err(err).
				Str("order_id", order.ID).
				Str("admin_id", a.ID).
				Msg("Failed to send WA large-tx notification")
		}
	}
}

// formatLargeTransactionNotification renders the owner-facing message for
// transaksi besar (≥ largeTransactionThreshold). Format mengikuti template
// notif transaksi yang sudah familiar untuk Bu Santi (header "Transaksi
// Baru", item 2-baris, total di bawah, waktu + ID di footer).
func formatLargeTransactionNotification(order *entity.Order, cashierName string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "*TOKO BAHAN KUE SANTI*\n")
	fmt.Fprintf(&b, "_Transaksi Baru_\n\n")
	fmt.Fprintf(&b, "Kasir: %s\n", cashierName)
	fmt.Fprintf(&b, "Pembayaran: %s\n", prettyPayment(order.Payment))
	if order.Member != nil && order.Member.Name != "" {
		fmt.Fprintf(&b, "Member: %s\n", order.Member.Name)
	} else if order.Customer != "" {
		fmt.Fprintf(&b, "Customer: %s\n", order.Customer)
	}

	if len(order.Items) > 0 {
		fmt.Fprintf(&b, "\nItem (%d):\n", len(order.Items))
		const maxShown = 10
		shown := order.Items
		hidden := 0
		if len(shown) > maxShown {
			shown = order.Items[:maxShown-1]
			hidden = len(order.Items) - len(shown)
		}
		for _, it := range shown {
			line := float64(it.Quantity) * it.UnitPrice
			fmt.Fprintf(&b, "• %s\n   %d × %s = %s\n",
				it.Name, it.Quantity, formatRupiah(it.UnitPrice), formatRupiah(line))
		}
		if hidden > 0 {
			fmt.Fprintf(&b, "• dan %d item lainnya\n", hidden)
		}
	}

	fmt.Fprintf(&b, "\n*TOTAL: %s*\n", formatRupiah(order.Total))

	jkt, _ := time.LoadLocation("Asia/Jakarta")
	fmt.Fprintf(&b, "\nWaktu: %s\n", order.CreatedAt.In(jkt).Format("02 Jan 2006, 15:04"))
	fmt.Fprintf(&b, "ID: #%s", shortID(order.ID))

	return b.String()
}

func formatRupiah(n float64) string {
	// Format with thousand-separator dots, Indonesian style.
	whole := int64(n)
	neg := whole < 0
	if neg {
		whole = -whole
	}
	s := fmt.Sprintf("%d", whole)
	var out strings.Builder
	if neg {
		out.WriteByte('-')
	}
	out.WriteString("Rp ")
	// Insert dots every 3 digits from the right.
	start := len(s) % 3
	if start > 0 {
		out.WriteString(s[:start])
		if len(s) > start {
			out.WriteByte('.')
		}
	}
	for i := start; i < len(s); i += 3 {
		out.WriteString(s[i : i+3])
		if i+3 < len(s) {
			out.WriteByte('.')
		}
	}
	return out.String()
}

func prettyPayment(p string) string {
	switch strings.ToLower(p) {
	case "cash":
		return "Tunai"
	case "transfer", "bank_transfer":
		return "Transfer"
	case "qris":
		return "QRIS"
	case "ewallet", "e-wallet":
		return "E-Wallet"
	case "card", "debit", "credit":
		return "Kartu"
	case "":
		return "-"
	}
	return p
}

// shortID returns the first 8 chars of a UUID — readable ID for the owner
// without exposing the full UUID.
func shortID(id string) string {
	if len(id) <= 8 {
		return strings.ToUpper(id)
	}
	return strings.ToUpper(id[:8])
}

func (s *OrderService) CancelOrder(id string) (*dto.OrderResponse, *dto.ApiError) {
	order, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}
	if order.Status != "completed" && order.Status != "pending" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order cannot be cancelled"}
	}

	tx := s.DB.Begin()

	// Restore stock
	for _, item := range order.Items {
		stockDelta := item.Quantity
		if item.UnitType == "box" {
			product, err := s.ProductRepo.FindByID(item.ProductID)
			if err == nil && product.QtyPerBox > 0 {
				stockDelta = item.Quantity * product.QtyPerBox
			}
		}
		tx.Model(&entity.Product{}).Where("id = ?", item.ProductID).
			Update("stock", gorm.Expr("stock + ?", stockDelta))
	}

	order.Status = "cancelled"
	if err := tx.Save(order).Error; err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to cancel order"}
	}

	tx.Commit()
	resp := s.toResponse(order)
	return &resp, nil
}

func (s *OrderService) consumeFIFO(tx *gorm.DB, productID string, qty int) {
	var batches []entity.StockBatch
	tx.Where("product_id = ? AND quantity > 0", productID).Order("received_at ASC").Find(&batches)

	remaining := qty
	for i := range batches {
		if remaining <= 0 {
			break
		}
		if batches[i].Quantity >= remaining {
			batches[i].Quantity -= remaining
			remaining = 0
		} else {
			remaining -= batches[i].Quantity
			batches[i].Quantity = 0
		}
		tx.Save(&batches[i])
	}
}

func (s *OrderService) GetRevenueStats() (float64, int64, *dto.ApiError) {
	revenue, count, err := s.Repo.GetRevenueStats()
	if err != nil {
		return 0, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to get stats"}
	}
	return revenue, count, nil
}

// Aggregate — hitung statistik untuk Reports + Dashboard server-side. Range
// completed orders di-aggregate jadi: total revenue/qty/count, top products
// sort by qty desc, member spending breakdown, payment method breakdown,
// per-cashier summary. Cegah FE load full orders array yang scale buruk.
//
// Empty from/to = all-time. Inclusive both sides. Filter status='completed'
// only (cancelled/refunded skip — konsisten dengan Reports/Dashboard convention).
func (s *OrderService) Aggregate(from, to string) (*dto.OrderAggregateResponse, *dto.ApiError) {
	orders, err := s.Repo.FindCompletedForAggregate(from, to)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch orders for aggregation")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to aggregate orders"}
	}

	resp := &dto.OrderAggregateResponse{
		From:        from,
		To:          to,
		TotalOrders: len(orders),
	}

	// Maps untuk agregat — finalize ke slice di akhir.
	type prodAgg struct {
		productID string
		name      string
		qty       int
		revenue   float64
	}
	type memberAgg struct {
		memberID  string
		name      string
		phone     string
		orders    int
		spend     float64
		savings   float64
		lastVisit time.Time
	}
	type cashierAgg struct {
		cashierID string
		name      string
		orders    int
		revenue   float64
		payments  map[string]*dto.AggregatePaymentBreakdown
	}

	products := map[string]*prodAgg{}
	members := map[string]*memberAgg{}
	paymentTotals := map[string]*dto.AggregatePaymentBreakdown{}
	cashiers := map[string]*cashierAgg{}

	for i := range orders {
		o := &orders[i]
		resp.TotalRevenue += o.Total

		// Items → top products + total qty + member savings
		for _, it := range o.Items {
			resp.TotalQty += it.Quantity
			if it.RegularPrice != nil && *it.RegularPrice > it.UnitPrice {
				resp.TotalMemberSaving += (*it.RegularPrice - it.UnitPrice) * float64(it.Quantity)
			}
			key := it.ProductID
			if key == "" {
				key = it.Name
			}
			pa, ok := products[key]
			if !ok {
				pa = &prodAgg{productID: it.ProductID, name: it.Name}
				products[key] = pa
			}
			pa.qty += it.Quantity
			pa.revenue += it.UnitPrice * float64(it.Quantity)
		}

		// Members (registered only)
		if o.Member != nil && o.Member.ID != "" {
			ma, ok := members[o.Member.ID]
			if !ok {
				ma = &memberAgg{
					memberID: o.Member.ID,
					name:     o.Member.Name,
					phone:    o.Member.Phone,
				}
				members[o.Member.ID] = ma
			}
			ma.orders++
			ma.spend += o.Total
			savings := 0.0
			for _, it := range o.Items {
				if it.RegularPrice != nil && *it.RegularPrice > it.UnitPrice {
					savings += (*it.RegularPrice - it.UnitPrice) * float64(it.Quantity)
				}
			}
			ma.savings += savings
			if o.CreatedAt.After(ma.lastVisit) {
				ma.lastVisit = o.CreatedAt
			}
		}

		// Payment breakdown — pakai split (Payments[]) kalau ada, else
		// fallback ke order.Payment string + Total.
		if len(o.Payments) > 0 {
			for _, p := range o.Payments {
				addPaymentBreakdown(paymentTotals, p.Method, p.Amount)
			}
		} else {
			addPaymentBreakdown(paymentTotals, o.Payment, o.Total)
		}

		// Per-cashier — kasir creator via order.CreatedBy
		if o.CreatedBy != "" {
			ca, ok := cashiers[o.CreatedBy]
			if !ok {
				name := "-"
				if u, err := s.AuthRepo.FindByID(o.CreatedBy); err == nil && u != nil {
					name = u.FullName
				}
				ca = &cashierAgg{
					cashierID: o.CreatedBy,
					name:      name,
					payments:  map[string]*dto.AggregatePaymentBreakdown{},
				}
				cashiers[o.CreatedBy] = ca
			}
			ca.orders++
			ca.revenue += o.Total
			if len(o.Payments) > 0 {
				for _, p := range o.Payments {
					addPaymentBreakdown(ca.payments, p.Method, p.Amount)
				}
			} else {
				addPaymentBreakdown(ca.payments, o.Payment, o.Total)
			}
		}
	}

	// Materialize: products → sort by qty desc
	resp.TopProducts = make([]dto.AggregateTopProduct, 0, len(products))
	for _, p := range products {
		avg := 0.0
		if p.qty > 0 {
			avg = p.revenue / float64(p.qty)
		}
		resp.TopProducts = append(resp.TopProducts, dto.AggregateTopProduct{
			ProductID: p.productID,
			Name:      p.name,
			Qty:       p.qty,
			Revenue:   p.revenue,
			AvgPrice:  avg,
		})
	}
	sort.Slice(resp.TopProducts, func(i, j int) bool {
		return resp.TopProducts[i].Qty > resp.TopProducts[j].Qty
	})

	// Members → sort by spend desc
	resp.Members = make([]dto.AggregateMember, 0, len(members))
	for _, m := range members {
		mr := dto.AggregateMember{
			MemberID: m.memberID,
			Name:     m.name,
			Phone:    m.phone,
			Orders:   m.orders,
			Spend:    m.spend,
			Savings:  m.savings,
		}
		if !m.lastVisit.IsZero() {
			mr.LastVisit = m.lastVisit.Format(time.RFC3339)
		}
		resp.Members = append(resp.Members, mr)
	}
	sort.Slice(resp.Members, func(i, j int) bool {
		return resp.Members[i].Spend > resp.Members[j].Spend
	})

	// Payment breakdown → slice
	resp.PaymentBreakdown = make([]dto.AggregatePaymentBreakdown, 0, len(paymentTotals))
	for _, p := range paymentTotals {
		resp.PaymentBreakdown = append(resp.PaymentBreakdown, *p)
	}
	sort.Slice(resp.PaymentBreakdown, func(i, j int) bool {
		return resp.PaymentBreakdown[i].Total > resp.PaymentBreakdown[j].Total
	})

	// Cashiers → sort by revenue desc
	resp.PerCashier = make([]dto.AggregateCashier, 0, len(cashiers))
	for _, c := range cashiers {
		pb := make([]dto.AggregatePaymentBreakdown, 0, len(c.payments))
		for _, p := range c.payments {
			pb = append(pb, *p)
		}
		sort.Slice(pb, func(i, j int) bool { return pb[i].Total > pb[j].Total })
		resp.PerCashier = append(resp.PerCashier, dto.AggregateCashier{
			CashierID:        c.cashierID,
			Name:             c.name,
			Orders:           c.orders,
			Revenue:          c.revenue,
			PaymentBreakdown: pb,
		})
	}
	sort.Slice(resp.PerCashier, func(i, j int) bool {
		return resp.PerCashier[i].Revenue > resp.PerCashier[j].Revenue
	})

	return resp, nil
}

func addPaymentBreakdown(m map[string]*dto.AggregatePaymentBreakdown, method string, amount float64) {
	if method == "" {
		method = "unknown"
	}
	p, ok := m[method]
	if !ok {
		p = &dto.AggregatePaymentBreakdown{Method: method}
		m[method] = p
	}
	p.Count++
	p.Total += amount
}

// primaryPaymentMethod picks the "headline" method for the orders.payment
// column (used for list filters). It returns the method of the largest-amount
// payment, so a Rp 50k cash + Rp 30k qris split is reported as "cash".
func primaryPaymentMethod(payments []dto.CreateOrderPaymentRequest) string {
	if len(payments) == 0 {
		return "cash"
	}
	best := payments[0]
	for _, p := range payments[1:] {
		if p.Amount > best.Amount {
			best = p
		}
	}
	return best.Method
}

// CreatePending — customer pesan online, kasir input ke POS tapi belum
// bayar. Stok TIDAK dipotong (hanya dipotong saat MarkAsPaid). WA invoice
// dengan rincian pembayaran dikirim ke customer_phone (kalau WA aktif).
func (s *OrderService) CreatePending(req dto.CreatePendingOrderRequest, userID string) (*dto.OrderResponse, *dto.ApiError) {
	if req.CustomerPhone == "" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Nomor HP customer wajib diisi untuk pesanan pending"}
	}

	// Hitung cash subtotal + validasi saldo poin kalau ada redeem.
	// Pending order tidak decrement saldo poin SEKARANG — itu di MarkAsPaid.
	// Validasi di sini hanya guard supaya tidak buat invoice yang nanti gagal.
	var pointsToRedeem int
	var cashSubtotal float64
	for _, item := range req.Items {
		lineTotal := item.UnitPrice * float64(item.Quantity)
		if item.RedeemWithPoints {
			pointsToRedeem += int(lineTotal)
		} else {
			cashSubtotal += lineTotal
		}
	}
	if pointsToRedeem > 0 {
		if req.MemberID == nil || *req.MemberID == "" {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Tebus barang dengan poin perlu pilih member"}
		}
		member, err := s.MemberRepo.FindByID(*req.MemberID)
		if err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Member tidak ditemukan"}
		}
		if member.Points < pointsToRedeem {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Saldo poin tidak cukup: butuh %d, tersedia %d", pointsToRedeem, member.Points)}
		}
	}

	tx := s.DB.Begin()

	order := &entity.Order{
		ID:                 uuid.New().String(),
		Subtotal:           req.Subtotal,
		PPNRate:            req.PPNRate,
		PPN:                req.PPN,
		Total:              cashSubtotal,
		Payment:            "cash", // placeholder; real method set at MarkAsPaid
		Status:             "pending",
		Customer:           req.Customer,
		CustomerPhone:      req.CustomerPhone,
		MemberID:           req.MemberID,
		OrderDiscountType:  req.OrderDiscountType,
		OrderDiscountValue: req.OrderDiscountValue,
		OrderDiscount:      req.OrderDiscount,
		CreatedBy:          userID,
	}

	for _, item := range req.Items {
		orderItem := entity.OrderItem{
			ID:                 uuid.New().String(),
			OrderID:            order.ID,
			ProductID:          item.ProductID,
			Name:               item.Name,
			Quantity:           item.Quantity,
			UnitType:           item.UnitType,
			UnitPrice:          item.UnitPrice,
			PurchasePrice:      item.PurchasePrice,
			RegularPrice:       item.RegularPrice,
			DiscountType:       item.DiscountType,
			DiscountValue:      item.DiscountValue,
			DiscountAmount:     item.DiscountAmount,
			RedeemedWithPoints: item.RedeemWithPoints,
		}
		if orderItem.UnitType == "" {
			orderItem.UnitType = "individual"
		}
		// Snapshot regular & purchase price (same as Create), but DO NOT
		// decrement stock — that happens in MarkAsPaid.
		product, err := s.ProductRepo.FindByID(item.ProductID)
		if err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Product not found: " + item.ProductID}
		}
		if orderItem.RegularPrice == nil {
			rp := product.SellingPrice
			if orderItem.UnitType == "box" && product.QtyPerBox > 0 {
				rp = product.SellingPrice * float64(product.QtyPerBox)
			}
			orderItem.RegularPrice = &rp
		}
		purchase := product.PurchasePrice
		if orderItem.UnitType == "box" && product.QtyPerBox > 0 {
			purchase = product.PurchasePrice * float64(product.QtyPerBox)
		}
		orderItem.PurchasePrice = purchase
		order.Items = append(order.Items, orderItem)
	}

	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		s.Log.Error().Err(err).Msg("Failed to create pending order")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create pending order"}
	}
	tx.Commit()

	created, _ := s.Repo.FindByID(order.ID)
	if created != nil {
		order = created
	}

	// Send WA invoice (bank transfer instructions) to customer — async.
	go s.sendPendingInvoiceWA(order, req.BankAccountID)

	resp := s.toResponse(order)
	return &resp, nil
}

// MarkAsPaid — pending order lunas. Jalankan pengecekan stok + potong stok +
// FIFO consume batch + kirim WA receipt ke customer + notif admin.
func (s *OrderService) MarkAsPaid(orderID string, req dto.MarkAsPaidRequest, userID string) (*dto.OrderResponse, *dto.ApiError) {
	order, err := s.Repo.FindByID(orderID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}
	if order.Status != "pending" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order is not pending"}
	}

	var paidSum float64
	for _, p := range req.Payments {
		paidSum += p.Amount
	}
	if paidSum+0.001 < order.Total {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Kurang bayar: total Rp %.0f, diterima Rp %.0f", order.Total, paidSum)}
	}

	tx := s.DB.Begin()

	// Now decrement stock (was not decremented at pending creation).
	for _, item := range order.Items {
		stockDelta := item.Quantity
		if item.UnitType == "box" {
			prod, _ := s.ProductRepo.FindByID(item.ProductID)
			if prod != nil && prod.QtyPerBox > 0 {
				stockDelta = item.Quantity * prod.QtyPerBox
			}
		}
		var prod entity.Product
		if err := tx.Where("id = ?", item.ProductID).First(&prod).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Product not found: " + item.ProductID}
		}
		if prod.Stock < stockDelta {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Insufficient stock for: " + prod.Name}
		}
		if err := tx.Model(&entity.Product{}).Where("id = ?", item.ProductID).
			Update("stock", gorm.Expr("stock - ?", stockDelta)).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to deduct stock"}
		}
		s.consumeFIFO(tx, item.ProductID, stockDelta)

		// Audit trail: stock-out movement record (sama dengan flow Create order).
		if err := tx.Create(&entity.StockMovement{
			ID:        uuid.New().String(),
			ProductID: item.ProductID,
			Type:      "out",
			Quantity:  stockDelta,
			UnitType:  item.UnitType,
			UnitPrice: item.UnitPrice,
			Reason:    "sale",
			Note:      "Sale (Pending Order #" + order.ID + " marked paid)",
			CreatedBy: userID,
			CreatedAt: time.Now(),
		}).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to record stock movement"}
		}
	}

	// Persist payment split
	for _, p := range req.Payments {
		if err := tx.Create(&entity.OrderPayment{
			ID:      uuid.New().String(),
			OrderID: order.ID,
			Method:  p.Method,
			Amount:  p.Amount,
		}).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save payment"}
		}
	}

	// Flip status + primary payment method on the order row.
	order.Status = "completed"
	order.Payment = primaryPaymentMethod(req.Payments)
	if err := tx.Save(order).Error; err != nil {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update order"}
	}

	// Member points: apply redeem (item flagged redeemed_with_points sudah
	// di-set saat CreatePending) + earn dari cash actual yang kelipatan 100k.
	// Pending order tidak decrement saldo saat created — efektif berlaku di
	// MarkAsPaid ini supaya consistent dengan stock decrement timing.
	var pointsToRedeem int
	var cashSubtotal float64
	for _, it := range order.Items {
		lineTotal := it.UnitPrice * float64(it.Quantity)
		if it.RedeemedWithPoints {
			pointsToRedeem += int(lineTotal)
		} else {
			cashSubtotal += lineTotal
		}
	}
	pointsEarned := calculateEarnedPoints(cashSubtotal)
	if order.MemberID != nil && *order.MemberID != "" && (pointsToRedeem > 0 || pointsEarned > 0) {
		if apiErr := s.applyPointsChange(tx, *order.MemberID, order.ID, userID, pointsToRedeem, pointsEarned); apiErr != nil {
			tx.Rollback()
			return nil, apiErr
		}
	}

	tx.Commit()

	reloaded, _ := s.Repo.FindByID(order.ID)
	if reloaded != nil {
		order = reloaded
	}

	// Owner request: jangan kirim struk WA ke customer setelah pending
	// dilunasi — customer sudah dapat invoice WA saat order pending dibuat.
	// Admin notif: hanya kalau transaksi besar (≥ largeTransactionThreshold).
	if order.Total >= largeTransactionThreshold {
		go s.sendLargeTransactionNotificationWA(order, userID)
	}

	resp := s.toResponse(order)
	return &resp, nil
}

// CancelPending — order pending dibatalkan. Stok tidak disentuh (memang
// belum pernah dipotong). Status → cancelled.
func (s *OrderService) CancelPending(orderID string, userID string) *dto.ApiError {
	order, err := s.Repo.FindByID(orderID)
	if err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}
	if order.Status != "pending" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Hanya pesanan pending yang bisa dibatalkan"}
	}
	order.Status = "cancelled"
	if err := s.Repo.Update(order); err != nil {
		s.Log.Error().Err(err).Msg("Failed to cancel pending order")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to cancel order"}
	}
	_ = userID // reserved for future audit log
	return nil
}

// ResendPendingInvoiceWA — kirim ulang rincian pembayaran ke customer_phone.
func (s *OrderService) ResendPendingInvoiceWA(orderID string, bankAccountID string) *dto.ApiError {
	if s.WA == nil || !s.WA.Enabled() {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "WhatsApp receipt is disabled"}
	}
	order, err := s.Repo.FindByID(orderID)
	if err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
	}
	if order.Status != "pending" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order is not pending"}
	}
	if order.CustomerPhone == "" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order has no customer phone"}
	}
	s.sendPendingInvoiceWASync(order, bankAccountID)
	return nil
}

// sendPendingInvoiceWA — fire-and-forget; sendPendingInvoiceWASync caller.
func (s *OrderService) sendPendingInvoiceWA(order *entity.Order, bankAccountID string) {
	s.sendPendingInvoiceWASync(order, bankAccountID)
}

func (s *OrderService) sendPendingInvoiceWASync(order *entity.Order, bankAccountID string) {
	if s.WA == nil || !s.WA.Enabled() {
		return
	}
	if order.CustomerPhone == "" {
		return
	}
	storeName := "Toko Bahan Kue Santi"
	storeAddress, storePhone := "", ""
	var bankLine string
	if settings, err := s.SettingsRepo.Get(); err == nil && settings != nil {
		if settings.StoreName != "" {
			storeName = settings.StoreName
		}
		storeAddress = settings.StoreAddress
		storePhone = settings.StorePhone
		// BankAccounts is JSON; caller passes an ID — pick matching account or first.
		bankLine = pickBankAccountLine(settings.BankAccounts, bankAccountID)
	}
	text := whatsapp.FormatPendingInvoice(order, storeName, storeAddress, storePhone, bankLine)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.WA.SendText(ctx, order.CustomerPhone, text); err != nil {
		s.Log.Warn().Err(err).Str("order_id", order.ID).Msg("Pending invoice WA failed")
	}
}

// pickBankAccountLine — pick a bank account matching the given ID, or the
// first available if id is empty / not found. Returns "" if no account
// configured (caller shows a "bayar di kasir" fallback in the WA message).
func pickBankAccountLine(accounts []entity.BankAccount, id string) string {
	if len(accounts) == 0 {
		return ""
	}
	render := func(a entity.BankAccount) string {
		return strings.TrimSpace(fmt.Sprintf("%s %s a.n. %s", a.BankName, a.AccountNumber, a.AccountHolder))
	}
	if id != "" {
		for _, a := range accounts {
			if a.ID == id {
				return render(a)
			}
		}
	}
	return render(accounts[0])
}

func (s *OrderService) toResponse(o *entity.Order) dto.OrderResponse {
	resp := dto.OrderResponse{
		ID:                 o.ID,
		Subtotal:           o.Subtotal,
		PPNRate:            o.PPNRate,
		PPN:                o.PPN,
		Total:              o.Total,
		Payment:            o.Payment,
		Status:             o.Status,
		Customer:           o.Customer,
		CustomerPhone:      o.CustomerPhone,
		MemberID:           o.MemberID,
		PaymentProof:       o.PaymentProof,
		OrderDiscountType:  o.OrderDiscountType,
		OrderDiscountValue: o.OrderDiscountValue,
		OrderDiscount:      o.OrderDiscount,
		CreatedBy:          o.CreatedBy,
		CreatedAt:          o.CreatedAt.Format(time.RFC3339),
	}

	if o.Member != nil {
		resp.Member = &dto.OrderMemberInfo{
			ID:    o.Member.ID,
			Name:  o.Member.Name,
			Phone: o.Member.Phone,
		}
	}

	var savings float64
	var pointsUsed int
	for _, item := range o.Items {
		if item.RegularPrice != nil && *item.RegularPrice > item.UnitPrice {
			savings += (*item.RegularPrice - item.UnitPrice) * float64(item.Quantity)
		}
		if item.RedeemedWithPoints {
			pointsUsed += int(item.UnitPrice * float64(item.Quantity))
		}
		resp.Items = append(resp.Items, dto.OrderItemResponse{
			ID:                 item.ID,
			ProductID:          item.ProductID,
			Name:               item.Name,
			Quantity:           item.Quantity,
			UnitType:           item.UnitType,
			UnitPrice:          item.UnitPrice,
			PurchasePrice:      item.PurchasePrice,
			RegularPrice:       item.RegularPrice,
			DiscountType:       item.DiscountType,
			DiscountValue:      item.DiscountValue,
			DiscountAmount:     item.DiscountAmount,
			RedeemedWithPoints: item.RedeemedWithPoints,
		})
	}
	if o.MemberID != nil && savings > 0 {
		resp.MemberSavings = savings
	}
	resp.PointsUsed = pointsUsed
	// PointsEarned = lookup movement type=earn linked to this order. Best-
	// effort: query failure leaves it 0 — display-only field.
	if o.MemberID != nil {
		var earned struct{ Points int }
		s.DB.Model(&entity.MemberPointMovement{}).
			Select("COALESCE(SUM(points), 0) as points").
			Where("order_id = ? AND type = 'earn'", o.ID).
			Scan(&earned)
		resp.PointsEarned = earned.Points
	}
	for _, p := range o.Payments {
		resp.Payments = append(resp.Payments, dto.OrderPaymentResponse{
			ID:     p.ID,
			Method: p.Method,
			Amount: p.Amount,
		})
	}
	return resp
}
