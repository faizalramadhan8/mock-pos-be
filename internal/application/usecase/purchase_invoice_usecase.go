package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type PurchaseInvoiceService struct {
	Log         *zerolog.Logger
	DB          *gorm.DB
	Repo        *repository.PurchaseInvoiceRepository
	ProductRepo *repository.ProductRepository
}

func NewPurchaseInvoiceService(ctx context.Context, db *gorm.DB) *PurchaseInvoiceService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &PurchaseInvoiceService{
		Log:         logger,
		DB:          db,
		Repo:        repository.NewPurchaseInvoiceRepository(ctx, db),
		ProductRepo: repository.NewProductRepository(ctx, db),
	}
}

// Create — atomic transaction: insert invoice + N items + N batches (kalau
// ED) + N movements (type='in', reason='restock') + update products.stock
// per item. Kalau salah satu gagal, semua di-rollback.
func (s *PurchaseInvoiceService) Create(req dto.CreatePurchaseInvoiceRequest, userID string) (*dto.PurchaseInvoiceResponse, *dto.ApiError) {
	// Parse invoice_date (default today)
	invoiceDate := time.Now()
	if req.InvoiceDate != "" {
		if t, err := time.Parse("2006-01-02", req.InvoiceDate); err == nil {
			invoiceDate = t
		}
	}

	// Parse / compute due_date
	var dueDate *time.Time
	if req.DueDate != "" {
		if t, err := time.Parse("2006-01-02", req.DueDate); err == nil {
			dueDate = &t
		}
	} else {
		// Auto-calc dari payment_terms
		if days := parseNetDays(req.PaymentTerms); days > 0 {
			d := invoiceDate.AddDate(0, 0, days)
			dueDate = &d
		}
		// COD → due_date = invoice_date
		if req.PaymentTerms == "COD" {
			dueDate = &invoiceDate
		}
	}

	tx := s.DB.Begin()

	invoice := &entity.PurchaseInvoice{
		ID:             uuid.New().String(),
		InvoiceNumber:  strings.TrimSpace(req.InvoiceNumber),
		SupplierID:     req.SupplierID,
		InvoiceDate:    invoiceDate,
		DueDate:        dueDate,
		PaymentTerms:   req.PaymentTerms,
		PaymentStatus:  paymentStatusFromTerms(req.PaymentTerms),
		SubtotalAmount: req.SubtotalAmount,
		PPNAmount:      req.PPNAmount,
		TotalAmount:    req.TotalAmount,
		Note:           req.Note,
		CreatedBy:      userID,
	}
	// COD → langsung paid_at
	if invoice.PaymentStatus == "paid" {
		now := time.Now()
		invoice.PaidAt = &now
	}

	if err := tx.Create(invoice).Error; err != nil {
		tx.Rollback()
		s.Log.Error().Err(err).Msg("Failed to create purchase invoice")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create invoice"}
	}

	for _, itemReq := range req.Items {
		product, err := s.ProductRepo.FindByID(itemReq.ProductID)
		if err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Product not found: " + itemReq.ProductID}
		}

		// Konversi quantity: box → individual count
		qtyIndividual := itemReq.Quantity
		unitType := itemReq.UnitType
		if unitType == "" {
			unitType = "individual"
		}
		if unitType == "box" && product.QtyPerBox > 0 {
			qtyIndividual = itemReq.Quantity * product.QtyPerBox
		}

		var expiryPtr *time.Time
		if itemReq.ExpiryDate != "" {
			if t, err := time.Parse("2006-01-02", itemReq.ExpiryDate); err == nil {
				expiryPtr = &t
			}
		}

		// Create batch (kalau ada ED, untuk FIFO tracking)
		var batchID *string
		if expiryPtr != nil {
			batchNumber := genBatchNumber(invoice.InvoiceDate)
			edStr := expiryPtr.Format("2006-01-02")
			batch := &entity.StockBatch{
				ID:          uuid.New().String(),
				ProductID:   itemReq.ProductID,
				Quantity:    qtyIndividual,
				ExpiryDate:  &edStr,
				BatchNumber: batchNumber,
				Note:        "From PO #" + shortID(invoice.ID),
			}
			if err := tx.Create(batch).Error; err != nil {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create batch"}
			}
			batchID = &batch.ID
		}

		// Create stock_movement record (audit trail)
		edStrPtr := (*string)(nil)
		if expiryPtr != nil {
			s := expiryPtr.Format("2006-01-02")
			edStrPtr = &s
		}
		supplierIDPtr := &req.SupplierID
		movement := &entity.StockMovement{
			ID:         uuid.New().String(),
			ProductID:  itemReq.ProductID,
			Type:       "in",
			Quantity:   qtyIndividual,
			UnitType:   unitType,
			UnitPrice:  itemReq.UnitPrice,
			Reason:     "restock",
			Note:       "PO #" + shortID(invoice.ID) + " — " + req.SupplierID,
			ExpiryDate: edStrPtr,
			SupplierID: supplierIDPtr,
			CreatedBy:  userID,
		}
		if err := tx.Create(movement).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create movement"}
		}

		// Increment products.stock
		if err := tx.Model(&entity.Product{}).Where("id = ?", itemReq.ProductID).
			Update("stock", gorm.Expr("stock + ?", qtyIndividual)).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update stock"}
		}

		// Insert invoice item with batch + movement reference
		movID := movement.ID
		item := &entity.PurchaseInvoiceItem{
			ID:                uuid.New().String(),
			PurchaseInvoiceID: invoice.ID,
			ProductID:         itemReq.ProductID,
			Quantity:          qtyIndividual,
			UnitType:          unitType,
			UnitPrice:         itemReq.UnitPrice,
			ExpiryDate:        expiryPtr,
			BatchID:           batchID,
			MovementID:        &movID,
			Note:              itemReq.Note,
		}
		if err := tx.Create(item).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create invoice item"}
		}
	}

	if err := tx.Commit().Error; err != nil {
		s.Log.Error().Err(err).Msg("Failed to commit purchase invoice transaction")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save invoice"}
	}

	created, _ := s.Repo.FindByID(invoice.ID)
	if created != nil {
		invoice = created
	}
	resp := s.toResponse(invoice)
	return &resp, nil
}

func (s *PurchaseInvoiceService) GetAll(status, supplierID, from, to string, page, limit int) ([]dto.PurchaseInvoiceResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	invoices, total, err := s.Repo.FindAll(status, supplierID, from, to, limit, offset)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch purchase invoices")
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch invoices"}
	}

	out := make([]dto.PurchaseInvoiceResponse, 0, len(invoices))
	for _, inv := range invoices {
		out = append(out, s.toResponse(&inv))
	}
	return out, total, nil
}

func (s *PurchaseInvoiceService) GetByID(id string) (*dto.PurchaseInvoiceResponse, *dto.ApiError) {
	inv, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Invoice not found"}
	}
	resp := s.toResponse(inv)
	return &resp, nil
}

// MarkAsPaid — flip status ke 'paid' + set paid_at. Idempotent: kalau sudah
// paid, return error 400 supaya UI tidak bingung.
func (s *PurchaseInvoiceService) MarkAsPaid(id string) (*dto.PurchaseInvoiceResponse, *dto.ApiError) {
	inv, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Invoice not found"}
	}
	if inv.PaymentStatus == "paid" {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Invoice sudah lunas"}
	}

	now := time.Now()
	inv.PaymentStatus = "paid"
	inv.PaidAt = &now
	if err := s.Repo.Update(inv); err != nil {
		s.Log.Error().Err(err).Msg("Failed to mark invoice as paid")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update invoice"}
	}

	resp := s.toResponse(inv)
	return &resp, nil
}

// Delete — soft delete invoice. Stock NOT reverted (data lama tetap sesuai
// realita; kalau salah input, edit/replace pakai opname adjustment). Cascade
// delete dengan items via FK ON DELETE CASCADE di DB, tapi karena soft delete
// via GORM, items tetap ada — deleted_at hanya di header.
func (s *PurchaseInvoiceService) Delete(id string) *dto.ApiError {
	if _, err := s.Repo.FindByID(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Invoice not found"}
	}
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete invoice")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete invoice"}
	}
	return nil
}

func (s *PurchaseInvoiceService) toResponse(inv *entity.PurchaseInvoice) dto.PurchaseInvoiceResponse {
	resp := dto.PurchaseInvoiceResponse{
		ID:             inv.ID,
		InvoiceNumber:  inv.InvoiceNumber,
		SupplierID:     inv.SupplierID,
		InvoiceDate:    inv.InvoiceDate.Format(time.RFC3339),
		PaymentTerms:   inv.PaymentTerms,
		PaymentStatus:  inv.PaymentStatus,
		SubtotalAmount: inv.SubtotalAmount,
		PPNAmount:      inv.PPNAmount,
		TotalAmount:    inv.TotalAmount,
		Note:           inv.Note,
		CreatedBy:      inv.CreatedBy,
		CreatedAt:      inv.CreatedAt.Format(time.RFC3339),
		Items:          []dto.PurchaseInvoiceItemResponse{},
	}
	if inv.DueDate != nil {
		s := inv.DueDate.Format(time.RFC3339)
		resp.DueDate = &s
	}
	if inv.PaidAt != nil {
		s := inv.PaidAt.Format(time.RFC3339)
		resp.PaidAt = &s
	}
	if inv.ReminderSentAt != nil {
		s := inv.ReminderSentAt.Format(time.RFC3339)
		resp.ReminderSentAt = &s
	}
	if inv.Supplier != nil {
		resp.Supplier = &dto.SupplierResponse{
			ID:      inv.Supplier.ID,
			Name:    inv.Supplier.Name,
			Phone:   inv.Supplier.Phone,
			Email:   inv.Supplier.Email,
			Address: inv.Supplier.Address,
		}
	}
	for _, it := range inv.Items {
		itemResp := dto.PurchaseInvoiceItemResponse{
			ID:                it.ID,
			PurchaseInvoiceID: it.PurchaseInvoiceID,
			ProductID:         it.ProductID,
			Quantity:          it.Quantity,
			UnitType:          it.UnitType,
			UnitPrice:         it.UnitPrice,
			BatchID:           it.BatchID,
			MovementID:        it.MovementID,
			Note:              it.Note,
		}
		if it.ExpiryDate != nil {
			s := it.ExpiryDate.Format("2006-01-02")
			itemResp.ExpiryDate = &s
		}
		if it.Product != nil {
			pr := dto.ProductResponse{
				ID:   it.Product.ID,
				SKU:  it.Product.SKU,
				Name: it.Product.Name,
			}
			itemResp.Product = &pr
		}
		resp.Items = append(resp.Items, itemResp)
	}
	return resp
}

// parseNetDays extracts the int N from "NET7" / "NET14" / "NET30" / dst.
// Returns 0 untuk COD (caller handles separately) atau format unknown.
func parseNetDays(terms string) int {
	if !strings.HasPrefix(terms, "NET") {
		return 0
	}
	days := 0
	for _, c := range terms[3:] {
		if c < '0' || c > '9' {
			return 0
		}
		days = days*10 + int(c-'0')
	}
	return days
}

// paymentStatusFromTerms — COD = paid (bayar langsung), NET* = unpaid.
func paymentStatusFromTerms(terms string) string {
	if terms == "COD" {
		return "paid"
	}
	return "unpaid"
}

// genBatchNumber — format B-YYYYMMDD-NNN dari invoice date + random suffix
// supaya batch tidak collide kalau banyak faktur masuk dalam 1 hari.
func genBatchNumber(invoiceDate time.Time) string {
	return "B-" + invoiceDate.Format("20060102") + "-" + uuid.New().String()[:4]
}
