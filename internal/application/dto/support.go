package dto

type CreateMemberRequest struct {
	Name  string `json:"name" validate:"required"`
	Phone string `json:"phone" validate:"required"`
}

type MemberResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	CreatedAt string `json:"created_at"`
}

type OpenCashSessionRequest struct {
	Date        string  `json:"date" validate:"required"`
	OpeningCash float64 `json:"opening_cash"`
	OpenedBy    string  `json:"opened_by" validate:"required"`
}

type CloseCashSessionRequest struct {
	ExpectedCash float64 `json:"expected_cash"`
	ActualCash   float64 `json:"actual_cash"`
	Difference   float64 `json:"difference"`
	Notes        string  `json:"notes"`
	ClosedBy     string  `json:"closed_by" validate:"required"`
}

type CashSessionResponse struct {
	ID           string  `json:"id"`
	Date         string  `json:"date"`
	OpeningCash  float64 `json:"opening_cash"`
	OpenedBy     string  `json:"opened_by"`
	OpenedAt     string  `json:"opened_at"`
	ExpectedCash float64 `json:"expected_cash"`
	ActualCash   float64 `json:"actual_cash"`
	Difference   float64 `json:"difference"`
	Notes        string  `json:"notes,omitempty"`
	ClosedBy     string  `json:"closed_by,omitempty"`
	ClosedAt     *string `json:"closed_at,omitempty"`
}

type CreateAuditEntryRequest struct {
	Action   string `json:"action" validate:"required"`
	UserID   string `json:"user_id" validate:"required"`
	UserName string `json:"user_name" validate:"required"`
	Details  string `json:"details"`
}

type AuditEntryResponse struct {
	ID        string `json:"id"`
	Action    string `json:"action"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	Details   string `json:"details,omitempty"`
	CreatedAt string `json:"created_at"`
}

type UpdateSettingsRequest struct {
	StoreName    string  `json:"store_name"`
	StoreAddress string  `json:"store_address"`
	StorePhone   string  `json:"store_phone"`
	PPNRate      float64 `json:"ppn_rate"`
	LabelWidth   int     `json:"label_width"`
	LabelHeight  int     `json:"label_height"`
}

type AddBankAccountRequest struct {
	BankName      string `json:"bank_name" validate:"required"`
	AccountNumber string `json:"account_number" validate:"required"`
	AccountHolder string `json:"account_holder" validate:"required"`
}

type SettingsResponse struct {
	ID           string                `json:"id"`
	StoreName    string                `json:"store_name"`
	StoreAddress string                `json:"store_address,omitempty"`
	StorePhone   string                `json:"store_phone,omitempty"`
	PPNRate      float64               `json:"ppn_rate"`
	LabelWidth   int                   `json:"label_width"`
	LabelHeight  int                   `json:"label_height"`
	BankAccounts []BankAccountResponse `json:"bank_accounts"`
}

type BankAccountResponse struct {
	ID            string `json:"id"`
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	AccountHolder string `json:"account_holder"`
}

type DashboardResponse struct {
	Revenue      float64              `json:"revenue"`
	OrderCount   int64                `json:"order_count"`
	ProductCount int64                `json:"product_count"`
	LowStockCount int64              `json:"low_stock_count"`
	RecentOrders []OrderResponse      `json:"recent_orders,omitempty"`
	LowStockItems []ProductResponse   `json:"low_stock_items,omitempty"`
	ExpiringBatches []StockBatchResponse `json:"expiring_batches,omitempty"`
}
