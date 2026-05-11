package repository

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type PurchaseInvoiceRepository struct {
	DB *gorm.DB
}

func NewPurchaseInvoiceRepository(ctx context.Context, db *gorm.DB) *PurchaseInvoiceRepository {
	return &PurchaseInvoiceRepository{DB: db}
}

// FindAll dengan filter status, supplier, date range. Order by invoice_date
// DESC (faktur terbaru di atas) untuk UX list view.
func (r *PurchaseInvoiceRepository) FindAll(status, supplierID, from, to string, limit, offset int) ([]entity.PurchaseInvoice, int64, error) {
	var invoices []entity.PurchaseInvoice
	var total int64

	q := r.DB.Model(&entity.PurchaseInvoice{})
	if status != "" && status != "all" {
		q = q.Where("payment_status = ?", status)
	}
	if supplierID != "" {
		q = q.Where("supplier_id = ?", supplierID)
	}
	if from != "" {
		q = q.Where("DATE(invoice_date) >= ?", from)
	}
	if to != "" {
		q = q.Where("DATE(invoice_date) <= ?", to)
	}
	q.Count(&total)

	if err := q.Preload("Supplier").Preload("Items").Preload("Items.Product").
		Order("invoice_date DESC").Limit(limit).Offset(offset).Find(&invoices).Error; err != nil {
		return nil, 0, err
	}
	return invoices, total, nil
}

func (r *PurchaseInvoiceRepository) FindByID(id string) (*entity.PurchaseInvoice, error) {
	var inv entity.PurchaseInvoice
	if err := r.DB.Preload("Supplier").Preload("Items").Preload("Items.Product").
		Where("id = ?", id).First(&inv).Error; err != nil {
		return nil, err
	}
	return &inv, nil
}

// FindDueToday cari faktur unpaid dengan due_date hari ini dan belum dikirim
// reminder. Dipakai oleh cron daily 09:00 WIB.
func (r *PurchaseInvoiceRepository) FindDueToday(today time.Time) ([]entity.PurchaseInvoice, error) {
	var invoices []entity.PurchaseInvoice
	if err := r.DB.Preload("Supplier").Preload("Items").Preload("Items.Product").
		Where("payment_status = 'unpaid' AND DATE(due_date) = DATE(?) AND reminder_sent_at IS NULL", today).
		Find(&invoices).Error; err != nil {
		return nil, err
	}
	return invoices, nil
}

func (r *PurchaseInvoiceRepository) Create(inv *entity.PurchaseInvoice) error {
	return r.DB.Create(inv).Error
}

func (r *PurchaseInvoiceRepository) Update(inv *entity.PurchaseInvoice) error {
	return r.DB.Save(inv).Error
}

// MarkReminderSent — set reminder_sent_at = now. Idempotent: kalau sudah set,
// tidak override (cron query sudah filter IS NULL jadi never re-update).
func (r *PurchaseInvoiceRepository) MarkReminderSent(tx *gorm.DB, id string, at time.Time) error {
	return tx.Model(&entity.PurchaseInvoice{}).Where("id = ?", id).
		Update("reminder_sent_at", at).Error
}

func (r *PurchaseInvoiceRepository) Delete(id string) error {
	return r.DB.Delete(&entity.PurchaseInvoice{}, "id = ?", id).Error
}

// SumPaidInPeriod — total nilai faktur LUNAS dengan tanggal bayar di periode.
// Dipakai untuk laporan Arus Kas (cash basis) — beda dari HPP yang dihitung
// per item yang laku. Fallback ke invoice_date kalau paid_at NULL (COD legacy).
func (r *PurchaseInvoiceRepository) SumPaidInPeriod(from, to string) (float64, error) {
	var result struct{ Total float64 }
	q := r.DB.Model(&entity.PurchaseInvoice{}).
		Where("payment_status = ?", "paid")
	if from != "" {
		q = q.Where("DATE(COALESCE(paid_at, invoice_date)) >= ?", from)
	}
	if to != "" {
		q = q.Where("DATE(COALESCE(paid_at, invoice_date)) <= ?", to)
	}
	err := q.Select("COALESCE(SUM(total_amount), 0) as total").Scan(&result).Error
	return result.Total, err
}

// SumUnpaidByInvoiceDate — total faktur BELUM lunas yang invoice_date-nya di
// periode. Info kewajiban cash flow yang masih tertunda.
func (r *PurchaseInvoiceRepository) SumUnpaidByInvoiceDate(from, to string) (float64, error) {
	var result struct{ Total float64 }
	q := r.DB.Model(&entity.PurchaseInvoice{}).
		Where("payment_status = ?", "unpaid")
	if from != "" {
		q = q.Where("DATE(invoice_date) >= ?", from)
	}
	if to != "" {
		q = q.Where("DATE(invoice_date) <= ?", to)
	}
	err := q.Select("COALESCE(SUM(total_amount), 0) as total").Scan(&result).Error
	return result.Total, err
}
