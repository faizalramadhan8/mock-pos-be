package dto

type CreateRefundRequest struct {
	OrderID string                   `json:"order_id" validate:"required"`
	Items   []CreateRefundItemRequest `json:"items" validate:"required,min=1"`
	Amount  float64                  `json:"amount"`
	Reason  string                   `json:"reason"`
}

type CreateRefundItemRequest struct {
	ProductID    string  `json:"product_id" validate:"required"`
	Name         string  `json:"name" validate:"required"`
	Quantity     int     `json:"quantity" validate:"required,min=1"`
	UnitType     string  `json:"unit_type"`
	UnitPrice    float64 `json:"unit_price"`
	RefundAmount float64 `json:"refund_amount"`
}

type RefundResponse struct {
	ID        string               `json:"id"`
	OrderID   string               `json:"order_id"`
	Items     []RefundItemResponse `json:"items"`
	Amount    float64              `json:"amount"`
	Reason    string               `json:"reason,omitempty"`
	CreatedBy string               `json:"created_by"`
	CreatedAt string               `json:"created_at"`
}

type RefundItemResponse struct {
	ID           string  `json:"id"`
	ProductID    string  `json:"product_id"`
	Name         string  `json:"name"`
	Quantity     int     `json:"quantity"`
	UnitType     string  `json:"unit_type"`
	UnitPrice    float64 `json:"unit_price"`
	RefundAmount float64 `json:"refund_amount"`
}
