package dto

// Sprint 4 Chunk 2 (31 Jul 2026) — Refund DTOs.

type EcomRefundCreateRequest struct {
	OrderID     string  `json:"order_id" validate:"required"`
	ComplaintID string  `json:"complaint_id"`               // optional
	Amount      float64 `json:"amount" validate:"required,gt=0"`
	Method      string  `json:"method" validate:"required,oneof=transfer_bank ewallet cash voucher other"`
	Note        string  `json:"note"`
	// Item mana yang restock. Kosong = customer keep barang.
	// Format: [{product_id: "xxx", qty: 2}]
	RestockItems []RefundRestockItem `json:"restock_items"`
}

type RefundRestockItem struct {
	ProductID string `json:"product_id" validate:"required"`
	Qty       int    `json:"qty" validate:"required,gt=0"`
}

type EcomRefundResponse struct {
	ID             string              `json:"id"`
	OrderID        string              `json:"order_id"`
	ComplaintID    string              `json:"complaint_id,omitempty"`
	Amount         float64             `json:"amount"`
	Method         string              `json:"method"`
	Note           string              `json:"note,omitempty"`
	RestockedItems []RefundRestockItem `json:"restocked_items"`
	RefundedBy     string              `json:"refunded_by"`
	RefundedByName string              `json:"refunded_by_name,omitempty"`
	RefundedAt     string              `json:"refunded_at"`
}
