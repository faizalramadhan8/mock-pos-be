package usecase

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type ExpenseService struct {
	Log         *zerolog.Logger
	DB          *gorm.DB
	Repo        *repository.ExpenseRepository
	OrderRepo   *repository.OrderRepository
	InvoiceRepo *repository.PurchaseInvoiceRepository
}

func NewExpenseService(ctx context.Context, db *gorm.DB) *ExpenseService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &ExpenseService{
		Log:         logger,
		DB:          db,
		Repo:        repository.NewExpenseRepository(ctx, db),
		OrderRepo:   repository.NewOrderRepository(ctx, db),
		InvoiceRepo: repository.NewPurchaseInvoiceRepository(ctx, db),
	}
}

// ─── Category ────────────────────────────────────────────────────────────

func (s *ExpenseService) ListCategories(includeInactive bool) ([]dto.ExpenseCategoryResponse, *dto.ApiError) {
	cats, err := s.Repo.FindAllCategories(includeInactive)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	out := make([]dto.ExpenseCategoryResponse, 0, len(cats))
	for _, c := range cats {
		out = append(out, toCategoryResp(c))
	}
	return out, nil
}

func (s *ExpenseService) CreateCategory(req dto.CreateExpenseCategoryRequest) (*dto.ExpenseCategoryResponse, *dto.ApiError) {
	c := entity.ExpenseCategory{
		ID:        uuid.New().String(),
		Name:      req.Name,
		IsSystem:  false,
		IsActive:  true,
		SortOrder: 500, // user-added category masuk di tengah, sebelum Lain-lain (999)
	}
	if err := s.Repo.CreateCategory(&c); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	resp := toCategoryResp(c)
	return &resp, nil
}

func (s *ExpenseService) UpdateCategory(id string, req dto.UpdateExpenseCategoryRequest) (*dto.ExpenseCategoryResponse, *dto.ApiError) {
	c, err := s.Repo.FindCategoryByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Category not found"}
	}
	// Sistem kategori: cuma boleh toggle is_active, nama dikunci supaya tidak
	// merusak referensi laporan historis.
	if !c.IsSystem {
		c.Name = req.Name
	}
	if req.IsActive != nil {
		c.IsActive = *req.IsActive
	}
	if err := s.Repo.UpdateCategory(c); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	resp := toCategoryResp(*c)
	return &resp, nil
}

// ─── Expense ─────────────────────────────────────────────────────────────

func (s *ExpenseService) List(req dto.ExpenseListRequest) ([]dto.ExpenseResponse, int64, *dto.ApiError) {
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	exps, total, err := s.Repo.FindAll(req.From, req.To, req.CategoryID, limit, offset)
	if err != nil {
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	out := make([]dto.ExpenseResponse, 0, len(exps))
	for _, e := range exps {
		out = append(out, toExpenseResp(e))
	}
	return out, total, nil
}

func (s *ExpenseService) Create(req dto.CreateExpenseRequest, userID string) (*dto.ExpenseResponse, *dto.ApiError) {
	date, err := time.Parse("2006-01-02", req.ExpenseDate)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Invalid expense_date format (use YYYY-MM-DD)"}
	}
	cat, err := s.Repo.FindCategoryByID(req.CategoryID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Category not found"}
	}
	pm := req.PaymentMethod
	if pm == "" {
		pm = "cash"
	}
	// Description optional — fallback ke nama kategori supaya list view tidak
	// kosong. Bu Santi sering pakai kategori spesifik (Listrik, Air) di mana
	// kategori itu sendiri sudah cukup self-explanatory.
	description := req.Description
	if description == "" {
		description = cat.Name
	}

	e := entity.Expense{
		ID:            uuid.New().String(),
		CategoryID:    req.CategoryID,
		ExpenseDate:   date,
		Description:   description,
		Amount:        req.Amount,
		EmployeeName:  req.EmployeeName,
		PaymentMethod: pm,
		Note:          req.Note,
		CreatedBy:     userID,
	}
	if err := s.Repo.Create(&e); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	// Re-fetch dengan preload category untuk response.
	saved, ferr := s.Repo.FindByID(e.ID)
	if ferr != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: ferr.Error()}
	}
	resp := toExpenseResp(*saved)
	return &resp, nil
}

func (s *ExpenseService) Update(id string, req dto.UpdateExpenseRequest) (*dto.ExpenseResponse, *dto.ApiError) {
	e, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Expense not found"}
	}
	date, perr := time.Parse("2006-01-02", req.ExpenseDate)
	if perr != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Invalid expense_date format (use YYYY-MM-DD)"}
	}
	cat, cerr := s.Repo.FindCategoryByID(req.CategoryID)
	if cerr != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Category not found"}
	}
	description := req.Description
	if description == "" {
		description = cat.Name
	}
	e.CategoryID = req.CategoryID
	e.ExpenseDate = date
	e.Description = description
	e.Amount = req.Amount
	e.EmployeeName = req.EmployeeName
	if req.PaymentMethod != "" {
		e.PaymentMethod = req.PaymentMethod
	}
	e.Note = req.Note
	if err := s.Repo.Update(e); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	saved, ferr := s.Repo.FindByID(e.ID)
	if ferr != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: ferr.Error()}
	}
	resp := toExpenseResp(*saved)
	return &resp, nil
}

func (s *ExpenseService) Delete(id string) *dto.ApiError {
	if _, err := s.Repo.FindByID(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Expense not found"}
	}
	if err := s.Repo.Delete(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	return nil
}

// ProfitLoss — kalkulasi Laporan Laba Rugi periode tertentu.
// Revenue + COGS dihitung dari order_items dengan status=completed.
// COGS pakai snapshot purchase_price (akurat historis — kalau harga modal
// berubah setelah sale, laporan masa lalu tetap valid).
func (s *ExpenseService) ProfitLoss(from, to string) (*dto.ProfitLossResponse, *dto.ApiError) {
	orders, err := s.OrderRepo.FindCompletedForAggregate(from, to)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	var revenue, cogs float64
	for _, o := range orders {
		for _, it := range o.Items {
			revenue += it.UnitPrice * float64(it.Quantity)
			cogs += it.PurchasePrice * float64(it.Quantity)
		}
	}

	expTotal, _, eerr := s.Repo.SumTotal(from, to)
	if eerr != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: eerr.Error()}
	}
	rows, berr := s.Repo.SumByCategory(from, to)
	if berr != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: berr.Error()}
	}
	breakdown := make([]dto.ExpenseCategoryBreakdown, 0, len(rows))
	for _, r := range rows {
		breakdown = append(breakdown, dto.ExpenseCategoryBreakdown{
			CategoryID:   r.CategoryID,
			CategoryName: r.CategoryName,
			Total:        r.Total,
			Count:        r.Count,
		})
	}

	// Cash flow — pembelian supplier yang sudah dibayar di periode + faktur
	// tempo yang masih jalan. Best-effort: kalau query gagal, set 0 supaya
	// laporan utama tetap render.
	supplierPaid, perr := s.InvoiceRepo.SumPaidInPeriod(from, to)
	if perr != nil {
		s.Log.Warn().Err(perr).Msg("SumPaidInPeriod failed; defaulting to 0")
		supplierPaid = 0
	}
	supplierUnpaid, uerr := s.InvoiceRepo.SumUnpaidByInvoiceDate(from, to)
	if uerr != nil {
		s.Log.Warn().Err(uerr).Msg("SumUnpaidByInvoiceDate failed; defaulting to 0")
		supplierUnpaid = 0
	}
	cashOut := supplierPaid + expTotal

	return &dto.ProfitLossResponse{
		From:             from,
		To:               to,
		Revenue:          revenue,
		COGS:             cogs,
		GrossProfit:      revenue - cogs,
		ExpenseTotal:     expTotal,
		ExpenseBreakdown: breakdown,
		NetProfit:        revenue - cogs - expTotal,
		TotalOrders:      len(orders),
		SupplierPaid:     supplierPaid,
		SupplierUnpaid:   supplierUnpaid,
		CashOutTotal:     cashOut,
		CashDiff:         revenue - cashOut,
	}, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────

func toCategoryResp(c entity.ExpenseCategory) dto.ExpenseCategoryResponse {
	return dto.ExpenseCategoryResponse{
		ID:        c.ID,
		Name:      c.Name,
		IsSystem:  c.IsSystem,
		IsActive:  c.IsActive,
		SortOrder: c.SortOrder,
	}
}

func toExpenseResp(e entity.Expense) dto.ExpenseResponse {
	resp := dto.ExpenseResponse{
		ID:            e.ID,
		CategoryID:    e.CategoryID,
		ExpenseDate:   e.ExpenseDate.Format("2006-01-02"),
		Description:   e.Description,
		Amount:        e.Amount,
		EmployeeName:  e.EmployeeName,
		PaymentMethod: e.PaymentMethod,
		Note:          e.Note,
		CreatedBy:     e.CreatedBy,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
	}
	if e.Category != nil {
		c := toCategoryResp(*e.Category)
		resp.Category = &c
	}
	return resp
}
