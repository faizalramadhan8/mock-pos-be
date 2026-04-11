package dto

type CreateStockMovementRequest struct {
	ProductID     string  `json:"product_id" validate:"required"`
	Type          string  `json:"type" validate:"required,oneof=in out"`
	Quantity      int     `json:"quantity" validate:"required,min=1"`
	UnitType      string  `json:"unit_type"`
	UnitPrice     float64 `json:"unit_price"`
	Note          string  `json:"note"`
	ExpiryDate    string  `json:"expiry_date"`
	SupplierID    string  `json:"supplier_id"`
	PaymentTerms  string  `json:"payment_terms"`
	DueDate       string  `json:"due_date"`
	PaymentStatus string  `json:"payment_status"`
	BatchNumber   string  `json:"batch_number"`
}

type UpdatePaymentStatusRequest struct {
	PaymentStatus string `json:"payment_status" validate:"required,oneof=paid unpaid"`
}

type StockMovementResponse struct {
	ID            string           `json:"id"`
	ProductID     string           `json:"product_id"`
	Product       *ProductResponse `json:"product,omitempty"`
	Type          string           `json:"type"`
	Quantity      int              `json:"quantity"`
	UnitType      string           `json:"unit_type"`
	UnitPrice     float64          `json:"unit_price"`
	Note          string           `json:"note,omitempty"`
	ExpiryDate    *string          `json:"expiry_date,omitempty"`
	SupplierID    *string          `json:"supplier_id,omitempty"`
	PaymentTerms  string           `json:"payment_terms,omitempty"`
	DueDate       *string          `json:"due_date,omitempty"`
	PaymentStatus string           `json:"payment_status,omitempty"`
	CreatedBy     string           `json:"created_by"`
	CreatedAt     string           `json:"created_at"`
}

type StockBatchResponse struct {
	ID          string           `json:"id"`
	ProductID   string           `json:"product_id"`
	Product     *ProductResponse `json:"product,omitempty"`
	Quantity    int              `json:"quantity"`
	ExpiryDate  *string          `json:"expiry_date,omitempty"`
	ReceivedAt  string           `json:"received_at"`
	Note        string           `json:"note,omitempty"`
	BatchNumber string           `json:"batch_number"`
}

type ConsumeFIFORequest struct {
	ProductID string `json:"product_id" validate:"required"`
	Quantity  int    `json:"quantity" validate:"required,min=1"`
}
