package dto

type CreateOrderRequest struct {
	Items              []CreateOrderItemRequest   `json:"items" validate:"required,min=1"`
	Subtotal           float64                    `json:"subtotal"`
	PPNRate            float64                    `json:"ppn_rate"`
	PPN                float64                    `json:"ppn"`
	Total              float64                    `json:"total"`
	Payment            string                     `json:"payment" validate:"required,oneof=cash card transfer qris"`
	// Payments is the split-payment breakdown. When present, sum of amounts
	// must be >= Total. When absent, the full Total is treated as paid with
	// the single Payment method (backward-compat with simple checkout).
	Payments           []CreateOrderPaymentRequest `json:"payments,omitempty"`
	Customer           string                     `json:"customer"`
	CustomerPhone      string                     `json:"customer_phone,omitempty"`
	MemberID           *string                    `json:"member_id,omitempty"`
	PaymentProof       string                     `json:"payment_proof"`
	OrderDiscountType  string                     `json:"order_discount_type"`
	OrderDiscountValue float64                    `json:"order_discount_value"`
	OrderDiscount      float64                    `json:"order_discount"`
}

type CreateOrderPaymentRequest struct {
	Method string  `json:"method" validate:"required,oneof=cash card transfer qris"`
	Amount float64 `json:"amount" validate:"required,gt=0"`
}

// MarkAsPaidRequest finalises a previously-created pending order. The
// caller supplies the actual payment split(s) the customer used.
type MarkAsPaidRequest struct {
	Payments []CreateOrderPaymentRequest `json:"payments" validate:"required,min=1,dive"`
}

// CreatePendingOrderRequest reuses most of CreateOrderRequest but never
// persists payments at creation time — they come later via MarkAsPaid.
type CreatePendingOrderRequest struct {
	Items              []CreateOrderItemRequest `json:"items" validate:"required,min=1"`
	Subtotal           float64                  `json:"subtotal"`
	PPNRate            float64                  `json:"ppn_rate"`
	PPN                float64                  `json:"ppn"`
	Total              float64                  `json:"total"`
	Customer           string                   `json:"customer"`
	CustomerPhone      string                   `json:"customer_phone" validate:"required"`
	MemberID           *string                  `json:"member_id,omitempty"`
	OrderDiscountType  string                   `json:"order_discount_type"`
	OrderDiscountValue float64                  `json:"order_discount_value"`
	OrderDiscount      float64                  `json:"order_discount"`
	// BankAccountID optional — picks one from settings to include in invoice.
	BankAccountID string `json:"bank_account_id,omitempty"`
}

type CreateOrderItemRequest struct {
	ProductID      string   `json:"product_id" validate:"required"`
	Name           string   `json:"name" validate:"required"`
	Quantity       int      `json:"quantity" validate:"required,min=1"`
	UnitType       string   `json:"unit_type"`
	UnitPrice      float64  `json:"unit_price"`
	PurchasePrice  float64  `json:"purchase_price,omitempty"`
	RegularPrice   *float64 `json:"regular_price,omitempty"`
	DiscountType   string   `json:"discount_type"`
	DiscountValue  float64  `json:"discount_value"`
	DiscountAmount float64  `json:"discount_amount"`
}

type OrderMemberInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type OrderResponse struct {
	ID                 string                 `json:"id"`
	Items              []OrderItemResponse    `json:"items"`
	Payments           []OrderPaymentResponse `json:"payments,omitempty"`
	Subtotal           float64                `json:"subtotal"`
	PPNRate            float64                `json:"ppn_rate"`
	PPN                float64                `json:"ppn"`
	Total              float64                `json:"total"`
	Payment            string                 `json:"payment"`
	Status             string                 `json:"status"`
	Customer           string                 `json:"customer,omitempty"`
	CustomerPhone      string                 `json:"customer_phone,omitempty"`
	MemberID           *string                `json:"member_id,omitempty"`
	Member             *OrderMemberInfo       `json:"member,omitempty"`
	MemberSavings      float64                `json:"member_savings,omitempty"`
	PaymentProof       string                 `json:"payment_proof,omitempty"`
	OrderDiscountType  string                 `json:"order_discount_type,omitempty"`
	OrderDiscountValue float64                `json:"order_discount_value,omitempty"`
	OrderDiscount      float64                `json:"order_discount,omitempty"`
	CreatedBy          string                 `json:"created_by"`
	CreatedAt          string                 `json:"created_at"`
}

type OrderPaymentResponse struct {
	ID     string  `json:"id"`
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
}

type OrderItemResponse struct {
	ID             string   `json:"id"`
	ProductID      string   `json:"product_id"`
	Name           string   `json:"name"`
	Quantity       int      `json:"quantity"`
	UnitType       string   `json:"unit_type"`
	UnitPrice      float64  `json:"unit_price"`
	PurchasePrice  float64  `json:"purchase_price,omitempty"`
	RegularPrice   *float64 `json:"regular_price,omitempty"`
	DiscountType   string   `json:"discount_type,omitempty"`
	DiscountValue  float64  `json:"discount_value,omitempty"`
	DiscountAmount float64  `json:"discount_amount,omitempty"`
}

type OrderListRequest struct {
	Status    string `query:"status"`
	StartDate string `query:"start_date"`
	EndDate   string `query:"end_date"`
	Search    string `query:"search"`
	Page      int    `query:"page"`
	Limit     int    `query:"limit"`
}
