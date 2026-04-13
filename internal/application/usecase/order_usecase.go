package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
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
}

func NewOrderService(ctx context.Context, db *gorm.DB) *OrderService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	wa, _ := ctx.Value(enum.WhatsAppCtxKey).(*whatsapp.Service)
	return &OrderService{
		Log:          logger,
		DB:           db,
		Repo:         repository.NewOrderRepository(ctx, db),
		ProductRepo:  repository.NewProductRepository(ctx, db),
		BatchRepo:    repository.NewStockBatchRepository(ctx, db),
		SettingsRepo: repository.NewSettingsRepository(ctx, db),
		AuthRepo:     repository.NewAuthRepository(ctx, db),
		WA:           wa,
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

	// Fire-and-forget WhatsApp receipt — never block checkout on WA failure
	go s.sendReceiptWA(order, userID)

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
	if order.Member == nil || order.Member.Phone == "" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Order is not linked to a member with a phone number"}
	}

	storeName := "BakeShop"
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
	if sendErr := s.WA.SendText(ctx, order.Member.Phone, text); sendErr != nil {
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
	if order.Member == nil || order.Member.Phone == "" {
		return
	}

	storeName := "BakeShop"
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
	if err := s.WA.SendText(ctx, order.Member.Phone, text); err != nil {
		s.Log.Warn().Err(err).
			Str("order_id", order.ID).
			Str("member_phone", order.Member.Phone).
			Msg("Failed to send WA receipt")
	}
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
