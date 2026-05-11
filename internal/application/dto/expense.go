package dto

type CreateExpenseRequest struct {
	CategoryID    string  `json:"category_id" validate:"required"`
	ExpenseDate   string  `json:"expense_date" validate:"required"` // YYYY-MM-DD
	Description   string  `json:"description" validate:"omitempty,max=255"`
	Amount        float64 `json:"amount" validate:"required,gt=0"`
	EmployeeName  string  `json:"employee_name"`
	PaymentMethod string  `json:"payment_method" validate:"omitempty,oneof=cash transfer qris"`
	Note          string  `json:"note"`
}

type UpdateExpenseRequest = CreateExpenseRequest

type ExpenseCategoryResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsSystem  bool   `json:"is_system"`
	IsActive  bool   `json:"is_active"`
	SortOrder int    `json:"sort_order"`
}

type CreateExpenseCategoryRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
}

type UpdateExpenseCategoryRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=100"`
	IsActive *bool  `json:"is_active"`
}

type ExpenseResponse struct {
	ID            string                   `json:"id"`
	CategoryID    string                   `json:"category_id"`
	Category      *ExpenseCategoryResponse `json:"category,omitempty"`
	ExpenseDate   string                   `json:"expense_date"`
	Description   string                   `json:"description"`
	Amount        float64                  `json:"amount"`
	EmployeeName  string                   `json:"employee_name,omitempty"`
	PaymentMethod string                   `json:"payment_method"`
	Note          string                   `json:"note,omitempty"`
	CreatedBy     string                   `json:"created_by"`
	CreatedAt     string                   `json:"created_at"`
}

type ExpenseListRequest struct {
	From       string `query:"from"`
	To         string `query:"to"`
	CategoryID string `query:"category_id"`
	Page       int    `query:"page"`
	Limit      int    `query:"limit"`
}

// ProfitLoss — Laporan Laba Rugi format yang familiar untuk Bu Santi.
//
//	Pendapatan            Rp X
//	- Modal Barang        Rp Y    (HPP, dari order_items.purchase_price)
//	= Laba Kotor          Rp X-Y
//	- Pengeluaran         Rp Z    (breakdown per kategori)
//	= UNTUNG BERSIH       Rp X-Y-Z
//
// Cancelled/refunded orders TIDAK ikut Revenue (cuma status=completed).
type ProfitLossRequest struct {
	From string `query:"from"` // YYYY-MM-DD inclusive
	To   string `query:"to"`   // YYYY-MM-DD inclusive
}

type ExpenseCategoryBreakdown struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Total        float64 `json:"total"`
	Count        int64   `json:"count"`
}

type ProfitLossResponse struct {
	From             string                     `json:"from"`
	To               string                     `json:"to"`
	Revenue          float64                    `json:"revenue"`            // SUM(order_items.unit_price * qty)
	COGS             float64                    `json:"cogs"`               // HPP — SUM(order_items.purchase_price * qty)
	GrossProfit      float64                    `json:"gross_profit"`       // Revenue - COGS
	ExpenseTotal     float64                    `json:"expense_total"`      // SUM(expenses.amount)
	ExpenseBreakdown []ExpenseCategoryBreakdown `json:"expense_breakdown"`  // per kategori
	NetProfit        float64                    `json:"net_profit"`         // GrossProfit - ExpenseTotal
	TotalOrders      int                        `json:"total_orders"`

	// ─── Cash Flow (Opsi B — view cash basis untuk Bu Santi) ─────────────
	// Berbeda dari NetProfit (accrual): ini real uang keluar masuk di periode.
	// SupplierPaid = pembelian dari faktur yang sudah lunas di periode ini.
	// CashOutTotal = SupplierPaid + ExpenseTotal (total uang keluar real).
	// CashDiff     = Revenue - CashOutTotal (selisih kas, BUKAN profit).
	SupplierPaid      float64 `json:"supplier_paid"`
	SupplierUnpaid    float64 `json:"supplier_unpaid"`     // info: faktur tempo blm lunas (cash flow risk)
	CashOutTotal      float64 `json:"cash_out_total"`
	CashDiff          float64 `json:"cash_diff"`
}
