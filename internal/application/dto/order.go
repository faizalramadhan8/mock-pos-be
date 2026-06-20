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
	// ClientRequestID — UUID per checkout attempt dari FE. BE simpan di
	// Redis 5 menit. Submit ulang dengan ID sama → return order existing
	// (idempotent). Cegah duplikat saat network slow / kasir double-click
	// / refresh-then-resubmit. Insiden 16 Jun 2026: 1 QRIS bayar 1x, sistem
	// catat 2 order (gap 17 menit) karena sinyal lambat + reload.
	ClientRequestID    string                     `json:"client_request_id,omitempty"`
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
	// ProductID: required UNLESS RedeemableItemID set (redeem row).
	// Validation: exactly one of {ProductID, RedeemableItemID} non-empty.
	ProductID        string  `json:"product_id"`
	RedeemableItemID *string `json:"redeemable_item_id,omitempty"`
	Name             string  `json:"name" validate:"required"`
	Quantity       int      `json:"quantity" validate:"required,min=1"`
	UnitType       string   `json:"unit_type"`
	UnitPrice      float64  `json:"unit_price"`
	PurchasePrice  float64  `json:"purchase_price,omitempty"`
	RegularPrice   *float64 `json:"regular_price,omitempty"`
	DiscountType   string   `json:"discount_type"`
	DiscountValue  float64  `json:"discount_value"`
	DiscountAmount float64  `json:"discount_amount"`
	// RedeemWithPoints: kalau true, item ini dibayar dari member.points
	// (tebus barang). UnitPrice × Quantity dipotong dari saldo poin.
	// Harga item tidak include cash subtotal — supaya tidak earn poin lagi.
	RedeemWithPoints bool `json:"redeem_with_points,omitempty"`
	// PriceSource: tag sumber harga (audit). Default 'regular'.
	// Values: 'regular' | 'member_price' | 'tier_all' | 'tier_member'.
	PriceSource string `json:"price_source,omitempty"`
	// TierID: kalau harga datang dari tier match, ID tier yang dipakai.
	TierID *string `json:"tier_id,omitempty"`
	// PaketCount + ExtraCount: snapshot paket logic.
	// Sum dari qty_satuan harus = paket_count*tier.min_qty + extra_count.
	PaketCount int `json:"paket_count,omitempty"`
	ExtraCount int `json:"extra_count,omitempty"`
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
	// PointsUsed = total poin yang ditebus pakai item RedeemedWithPoints
	// pada order ini. PointsEarned = poin yang didapat (kelipatan 100k).
	// Dua field ini snapshot di response untuk display di receipt + UI;
	// source of truth tetap di member_point_movements.
	PointsUsed   int `json:"points_used,omitempty"`
	PointsEarned int `json:"points_earned,omitempty"`
	CreatedBy    string `json:"created_by"`
	CreatedAt    string `json:"created_at"`
}

type OrderPaymentResponse struct {
	ID     string  `json:"id"`
	Method string  `json:"method"`
	Amount float64 `json:"amount"`
}

type OrderItemResponse struct {
	ID                 string   `json:"id"`
	ProductID          string   `json:"product_id,omitempty"`
	RedeemableItemID   *string  `json:"redeemable_item_id,omitempty"`
	Name               string   `json:"name"`
	Quantity           int      `json:"quantity"`
	UnitType           string   `json:"unit_type"`
	UnitPrice          float64  `json:"unit_price"`
	PurchasePrice      float64  `json:"purchase_price,omitempty"`
	RegularPrice       *float64 `json:"regular_price,omitempty"`
	DiscountType       string   `json:"discount_type,omitempty"`
	DiscountValue      float64  `json:"discount_value,omitempty"`
	DiscountAmount     float64  `json:"discount_amount,omitempty"`
	RedeemedWithPoints bool     `json:"redeemed_with_points,omitempty"`
	PriceSource        string   `json:"price_source,omitempty"`
	TierID             *string  `json:"tier_id,omitempty"`
	PaketCount         int      `json:"paket_count,omitempty"`
	ExtraCount         int      `json:"extra_count,omitempty"`
}

type OrderListRequest struct {
	Status    string `query:"status"`
	StartDate string `query:"start_date"`
	EndDate   string `query:"end_date"`
	Search    string `query:"search"`
	Page      int    `query:"page"`
	Limit     int    `query:"limit"`
}

// AggregateRequest — query untuk /orders/aggregate. Default: completed only,
// no date filter. Empty from/to = all-time aggregation.
type OrderAggregateRequest struct {
	From string `query:"from"` // YYYY-MM-DD inclusive
	To   string `query:"to"`   // YYYY-MM-DD inclusive
}

type AggregateTopProduct struct {
	ProductID  string  `json:"product_id"`
	Name       string  `json:"name"`
	Qty        int     `json:"qty"`
	Revenue    float64 `json:"revenue"`
	AvgPrice   float64 `json:"avg_price"`
}

type AggregateMember struct {
	MemberID  string  `json:"member_id"`
	Name      string  `json:"name"`
	Phone     string  `json:"phone,omitempty"`
	Orders    int     `json:"orders"`
	Spend     float64 `json:"spend"`
	Savings   float64 `json:"savings"`
	LastVisit string  `json:"last_visit,omitempty"`
}

type AggregatePaymentBreakdown struct {
	Method string  `json:"method"`
	Count  int     `json:"count"`
	Total  float64 `json:"total"`
}

type AggregateCashier struct {
	CashierID         string                      `json:"cashier_id"`
	Name              string                      `json:"name"`
	Orders            int                         `json:"orders"`
	Revenue           float64                     `json:"revenue"`
	PaymentBreakdown  []AggregatePaymentBreakdown `json:"payment_breakdown"`
}

type OrderAggregateResponse struct {
	From              string                      `json:"from"`
	To                string                      `json:"to"`
	TotalOrders       int                         `json:"total_orders"`
	TotalRevenue      float64                     `json:"total_revenue"`
	TotalQty          int                         `json:"total_qty"`
	TotalMemberSaving float64                     `json:"total_member_saving"`
	TopProducts       []AggregateTopProduct       `json:"top_products"`
	Members           []AggregateMember           `json:"members"`
	PaymentBreakdown  []AggregatePaymentBreakdown `json:"payment_breakdown"`
	PerCashier        []AggregateCashier          `json:"per_cashier"`
}
