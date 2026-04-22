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
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/whatsapp"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type OrderService struct {
	Log          *zerolog.Logger
	DB           *gorm.DB
	Repo         *repository.OrderRepository
	ProductRepo  *repository.ProductRepository
	BatchRepo    *repository.StockBatchRepository
	SettingsRepo *repository.SettingsRepository
	AuthRepo     *repository.AuthRepository
	WA           *whatsapp.Service
	Configs      *config.Config
}

func NewOrderService(ctx context.Context, db *gorm.DB) *OrderService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	wa, _ := ctx.Value(enum.WhatsAppCtxKey).(*whatsapp.Service)
	cfg, _ := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &OrderService{
		Log:          logger,
		DB:           db,
		Repo:         repository.NewOrderRepository(ctx, db),
		ProductRepo:  repository.NewProductRepository(ctx, db),
		BatchRepo:    repository.NewStockBatchRepository(ctx, db),
		SettingsRepo: repository.NewSettingsRepository(ctx, db),
		AuthRepo:     repository.NewAuthRepository(ctx, db),
		WA:           wa,
		Configs:      cfg,
	}
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
	tx := s.DB.Begin()

	order := &entity.Order{
		ID:                 uuid.New().String(),
		Subtotal:           req.Subtotal,
		PPNRate:            req.PPNRate,
		PPN:                req.PPN,
		Total:              req.Total,
		Payment:            req.Payment,
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

	for _, item := range req.Items {
		orderItem := entity.OrderItem{
			ID:             uuid.New().String(),
			OrderID:        order.ID,
			ProductID:      item.ProductID,
			Name:           item.Name,
			Quantity:       item.Quantity,
			UnitType:       item.UnitType,
			UnitPrice:      item.UnitPrice,
			PurchasePrice:  item.PurchasePrice,
			RegularPrice:   item.RegularPrice,
			DiscountType:   item.DiscountType,
			DiscountValue:  item.DiscountValue,
			DiscountAmount: item.DiscountAmount,
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
	}

	if err := tx.Create(order).Error; err != nil {
		tx.Rollback()
		s.Log.Error().Err(err).Msg("Failed to create order")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create order"}
	}

	tx.Commit()

	created, _ := s.Repo.FindByID(order.ID)
	if created != nil {
		order = created
	}

	// Fire-and-forget WhatsApp messages — never block checkout on WA failure.
	go s.sendReceiptWA(order, userID)
	go s.sendTransactionNotificationWA(order, userID)

	resp := s.toResponse(order)
	return &resp, nil
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
	if settings, sErr := s.SettingsRepo.Get(); sErr == nil && settings != nil && settings.StoreName != "" {
		storeName = settings.StoreName
	}
	cashierName := ""
	if u, uErr := s.AuthRepo.FindByID(userID); uErr == nil && u != nil {
		cashierName = u.FullName
	}
	text := whatsapp.FormatReceipt(order, storeName, cashierName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if sendErr := s.WA.SendText(ctx, phone, text); sendErr != nil {
		s.Log.Warn().Err(sendErr).Str("order_id", order.ID).Msg("WA resend failed")
		return &dto.ApiError{StatusCode: fiber.ErrBadGateway, Message: "Failed to send WhatsApp receipt: " + sendErr.Error()}
	}
	return nil
}

// sendReceiptWA dispatches a WhatsApp receipt to the member associated with
// the order. No-op if WA is disabled, order has no member, or member has no
// valid phone number. Runs in its own goroutine — all errors are logged.
func (s *OrderService) sendReceiptWA(order *entity.Order, userID string) {
	if s.WA == nil || !s.WA.Enabled() {
		return
	}

	// Prefer member phone; fall back to customer_phone (non-member who
	// provided a number at checkout).
	phone := ""
	if order.Member != nil && order.Member.Phone != "" {
		phone = order.Member.Phone
	} else if order.CustomerPhone != "" {
		phone = order.CustomerPhone
	}
	if phone == "" {
		return
	}

	storeName := "Toko Bahan Kue Santi"
	if settings, err := s.SettingsRepo.Get(); err == nil && settings != nil && settings.StoreName != "" {
		storeName = settings.StoreName
	}
	cashierName := ""
	if u, err := s.AuthRepo.FindByID(userID); err == nil && u != nil {
		cashierName = u.FullName
	}

	text := whatsapp.FormatReceipt(order, storeName, cashierName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := s.WA.SendText(ctx, phone, text); err != nil {
		s.Log.Warn().Err(err).
			Str("order_id", order.ID).
			Str("recipient_phone", phone).
			Msg("Failed to send WA receipt")
	}
}

// sendTransactionNotificationWA notifies every admin/superadmin (with a phone
// number on file) of a new transaction. Gated by WA_RECEIPT_ENABLED — the
// same toggle that governs the member receipt. Runs as a goroutine after
// checkout, never blocks the response.
func (s *OrderService) sendTransactionNotificationWA(order *entity.Order, userID string) {
	if s.WA == nil || !s.WA.Enabled() {
		return
	}
	admins, err := s.AuthRepo.FindAdmins()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to load admin list for transaction WA")
		return
	}
	if len(admins) == 0 {
		s.Log.Warn().Msg("No admin/superadmin with phone found; transaction notification skipped")
		return
	}

	cashierName := "-"
	if u, err := s.AuthRepo.FindByID(userID); err == nil && u != nil {
		cashierName = u.FullName
	}

	text := formatTransactionNotification(order, cashierName)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, a := range admins {
		if err := s.WA.SendText(ctx, a.PhoneNumber, text); err != nil {
			s.Log.Warn().Err(err).
				Str("order_id", order.ID).
				Str("admin_id", a.ID).
				Msg("Failed to send WA transaction notification")
		}
	}
}

// formatTransactionNotification renders the detail view for the owner's WA.
// Lists all items if ≤5, otherwise first 4 + "dan N lainnya".
func formatTransactionNotification(order *entity.Order, cashierName string) string {
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
	for _, item := range o.Items {
		if item.RegularPrice != nil && *item.RegularPrice > item.UnitPrice {
			savings += (*item.RegularPrice - item.UnitPrice) * float64(item.Quantity)
		}
		resp.Items = append(resp.Items, dto.OrderItemResponse{
			ID:             item.ID,
			ProductID:      item.ProductID,
			Name:           item.Name,
			Quantity:       item.Quantity,
			UnitType:       item.UnitType,
			UnitPrice:      item.UnitPrice,
			PurchasePrice:  item.PurchasePrice,
			RegularPrice:   item.RegularPrice,
			DiscountType:   item.DiscountType,
			DiscountValue:  item.DiscountValue,
			DiscountAmount: item.DiscountAmount,
		})
	}
	if o.MemberID != nil && savings > 0 {
		resp.MemberSavings = savings
	}
	return resp
}
