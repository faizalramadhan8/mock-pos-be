package entity

import (
	"time"

	"gorm.io/gorm"
)

// PurchaseInvoice — faktur pembelian dari supplier dengan multi-line items.
// 1 invoice = N items, atomic create di usecase. invoice_number opsional
// bebas (boleh kosong/duplicate, sesuai format supplier yang vary).
//
// reminder_sent_at: cron H-0 cek kalau NULL → kirim WA + set ke NOW. Cegah
// double-send kalau cron jalan ulang (e.g. server restart pada hari yg sama).
type PurchaseInvoice struct {
	ID              string         `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	InvoiceNumber   string         `gorm:"type:varchar(50);null" json:"invoice_number,omitempty"`
	SupplierID      string         `gorm:"type:varchar(36);not null" json:"supplier_id"`
	Supplier        *Supplier      `gorm:"foreignKey:SupplierID" json:"supplier,omitempty"`
	InvoiceDate     time.Time      `gorm:"not null;default:current_timestamp()" json:"invoice_date"`
	DueDate         *time.Time     `gorm:"null" json:"due_date,omitempty"`
	PaymentTerms    string         `gorm:"type:varchar(20);not null;default:'COD'" json:"payment_terms"`
	PaymentStatus   string         `gorm:"type:varchar(20);not null;default:'unpaid'" json:"payment_status"`
	PaidAt          *time.Time     `gorm:"null" json:"paid_at,omitempty"`
	SubtotalAmount  float64        `gorm:"type:decimal(15,2);not null;default:0" json:"subtotal_amount"`
	PPNAmount       float64        `gorm:"type:decimal(15,2);not null;default:0" json:"ppn_amount"`
	TotalAmount     float64        `gorm:"type:decimal(15,2);not null;default:0" json:"total_amount"`
	ReminderSentAt  *time.Time     `gorm:"null" json:"reminder_sent_at,omitempty"`
	Note            string         `gorm:"type:text;null" json:"note,omitempty"`
	CreatedBy       string         `gorm:"type:varchar(36);not null" json:"created_by"`
	CreatedAt       time.Time      `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt       time.Time      `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	Items []PurchaseInvoiceItem `gorm:"foreignKey:PurchaseInvoiceID" json:"items,omitempty"`
}

func (PurchaseInvoice) TableName() string { return "purchase_invoices" }

// PurchaseInvoiceItem — 1 baris produk dalam 1 faktur. quantity selalu
// dalam individual units (sesuai konvensi stock_movements). batch_id +
// movement_id di-populate saat Create — buat traceability kalau Bu Santi
// mau drill-down dari faktur ke batch/movement record.
type PurchaseInvoiceItem struct {
	ID                  string     `gorm:"type:varchar(36);primary_key;not null" json:"id"`
	PurchaseInvoiceID   string     `gorm:"type:varchar(36);not null" json:"purchase_invoice_id"`
	ProductID           string     `gorm:"type:varchar(36);not null" json:"product_id"`
	Product             *Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Quantity            int        `gorm:"type:int;not null;default:0" json:"quantity"`
	UnitType            string     `gorm:"type:varchar(20);not null;default:'individual'" json:"unit_type"`
	UnitPrice           float64    `gorm:"type:decimal(15,2);not null;default:0" json:"unit_price"`
	ExpiryDate          *time.Time `gorm:"type:date;null" json:"expiry_date,omitempty"`
	BatchID             *string    `gorm:"type:varchar(36);null" json:"batch_id,omitempty"`
	MovementID          *string    `gorm:"type:varchar(36);null" json:"movement_id,omitempty"`
	Note                string     `gorm:"type:text;null" json:"note,omitempty"`
	CreatedAt           time.Time  `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
}

func (PurchaseInvoiceItem) TableName() string { return "purchase_invoice_items" }
