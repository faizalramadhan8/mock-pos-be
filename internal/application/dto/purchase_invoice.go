package dto

// PurchaseInvoiceItemRequest — 1 baris produk dalam Create request.
// Quantity di-input dalam unit yang user pilih (box/individual); konversi
// ke individual count terjadi di usecase berdasarkan products.qty_per_box.
type PurchaseInvoiceItemRequest struct {
	ProductID  string  `json:"product_id" validate:"required"`
	Quantity   int     `json:"quantity" validate:"required,min=1"`
	UnitType   string  `json:"unit_type" validate:"omitempty,oneof=box individual"`
	UnitPrice  float64 `json:"unit_price" validate:"required,min=0"` // per-individual unit
	ExpiryDate string  `json:"expiry_date"`                          // YYYY-MM-DD opsional
	Note       string  `json:"note"`
}

type CreatePurchaseInvoiceRequest struct {
	InvoiceNumber  string                       `json:"invoice_number"` // opsional, bebas
	SupplierID     string                       `json:"supplier_id" validate:"required"`
	InvoiceDate    string                       `json:"invoice_date"`   // YYYY-MM-DD, default today
	PaymentTerms   string                       `json:"payment_terms" validate:"required,oneof=COD NET7 NET14 NET21 NET30 NET60 NET90"`
	DueDate        string                       `json:"due_date"`       // YYYY-MM-DD opsional (auto-calc dari invoice_date + payment_terms kalau kosong)
	SubtotalAmount float64                      `json:"subtotal_amount" validate:"min=0"`
	PPNAmount      float64                      `json:"ppn_amount" validate:"min=0"`
	TotalAmount    float64                      `json:"total_amount" validate:"required,min=0"`
	Note           string                       `json:"note"`
	Items          []PurchaseInvoiceItemRequest `json:"items" validate:"required,min=1,dive"`
}

type PurchaseInvoiceItemResponse struct {
	ID                string   `json:"id"`
	PurchaseInvoiceID string   `json:"purchase_invoice_id"`
	ProductID         string   `json:"product_id"`
	Product           *ProductResponse `json:"product,omitempty"`
	Quantity          int      `json:"quantity"`
	UnitType          string   `json:"unit_type"`
	UnitPrice         float64  `json:"unit_price"`
	ExpiryDate        *string  `json:"expiry_date,omitempty"`
	BatchID           *string  `json:"batch_id,omitempty"`
	MovementID        *string  `json:"movement_id,omitempty"`
	Note              string   `json:"note,omitempty"`
}

type PurchaseInvoiceResponse struct {
	ID              string                        `json:"id"`
	InvoiceNumber   string                        `json:"invoice_number,omitempty"`
	SupplierID      string                        `json:"supplier_id"`
	Supplier        *SupplierResponse             `json:"supplier,omitempty"`
	InvoiceDate     string                        `json:"invoice_date"`
	DueDate         *string                       `json:"due_date,omitempty"`
	PaymentTerms    string                        `json:"payment_terms"`
	PaymentStatus   string                        `json:"payment_status"`
	PaidAt          *string                       `json:"paid_at,omitempty"`
	SubtotalAmount  float64                       `json:"subtotal_amount"`
	PPNAmount       float64                       `json:"ppn_amount"`
	TotalAmount     float64                       `json:"total_amount"`
	ReminderSentAt  *string                       `json:"reminder_sent_at,omitempty"`
	Note            string                        `json:"note,omitempty"`
	CreatedBy       string                        `json:"created_by"`
	CreatedAt       string                        `json:"created_at"`
	Items           []PurchaseInvoiceItemResponse `json:"items"`
}
