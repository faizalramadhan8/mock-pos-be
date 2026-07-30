package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminStatsService — Sprint 3 #13. Aggregate metrics untuk Admin Dashboard
// Ecom widgets. Query cepat via GROUP BY / COUNT, tidak load full order rows.
type EcomAdminStatsService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAdminStatsService(ctx context.Context, db *gorm.DB) *EcomAdminStatsService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminStatsService{DB: db, Log: logger}
}

// GetDashboard — semua metrics dalam 1 request, cegah round-trip berkali-kali
// dari FE. Bu Santi buka dashboard sekali langsung dapet semua.
func (s *EcomAdminStatsService) GetDashboard() (*dto.EcomAdminDashboard, *dto.ApiError) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	dash := &dto.EcomAdminDashboard{}

	// Count per status — 1 query GROUP BY.
	var statusCounts []struct {
		EcomStatus string
		Cnt        int
	}
	s.DB.Model(&entity.Order{}).
		Select("ecom_status, COUNT(*) AS cnt").
		Where("order_source = 'ecom' AND deleted_at IS NULL").
		Group("ecom_status").Scan(&statusCounts)
	dash.StatusCounts = map[string]int{}
	for _, c := range statusCounts {
		dash.StatusCounts[c.EcomStatus] = c.Cnt
	}

	// Revenue hari ini + bulan ini — dari orders yang sudah paid+.
	// Include semua yang bukan cancelled/expired/pending_payment.
	activeStatus := []string{"paid", "processing", "shipped", "delivered", "completed"}

	var todayStats struct {
		Total float64
		Count int
	}
	s.DB.Model(&entity.Order{}).
		Select("COALESCE(SUM(total), 0) AS total, COUNT(*) AS count").
		Where("order_source = 'ecom' AND deleted_at IS NULL AND payment_paid_at >= ? AND ecom_status IN ?", todayStart, activeStatus).
		Scan(&todayStats)
	dash.TodayRevenue = todayStats.Total
	dash.TodayOrders = todayStats.Count

	var monthStats struct {
		Total float64
		Count int
	}
	s.DB.Model(&entity.Order{}).
		Select("COALESCE(SUM(total), 0) AS total, COUNT(*) AS count").
		Where("order_source = 'ecom' AND deleted_at IS NULL AND payment_paid_at >= ? AND ecom_status IN ?", monthStart, activeStatus).
		Scan(&monthStats)
	dash.MonthRevenue = monthStats.Total
	dash.MonthOrders = monthStats.Count

	// Top 5 produk terlaris bulan ini — GROUP BY product_id + SUM qty.
	var topProducts []struct {
		ProductID string
		Name      string
		QtySum    int
		Revenue   float64
	}
	s.DB.Table("order_items oi").
		Select("oi.product_id, oi.name, SUM(oi.quantity) AS qty_sum, SUM(oi.unit_price * oi.quantity) AS revenue").
		Joins("JOIN orders o ON o.id = oi.order_id").
		Where("o.order_source = 'ecom' AND o.deleted_at IS NULL AND o.payment_paid_at >= ? AND o.ecom_status IN ?", monthStart, activeStatus).
		Where("oi.product_id <> ''").
		Group("oi.product_id, oi.name").
		Order("qty_sum DESC").Limit(5).Scan(&topProducts)
	for _, tp := range topProducts {
		dash.TopProductsMonth = append(dash.TopProductsMonth, dto.EcomDashTopProduct{
			ProductID: tp.ProductID,
			Name:      tp.Name,
			QtySold:   tp.QtySum,
			Revenue:   tp.Revenue,
		})
	}

	// Total customer terdaftar (role=user).
	var custCount int64
	s.DB.Model(&entity.User{}).Where("role = 'user' AND is_active = 1").Count(&custCount)
	dash.TotalCustomers = int(custCount)

	// Complaint yang butuh perhatian — status open + in_review.
	var complaintPending int64
	s.DB.Model(&entity.EcomComplaint{}).Where("status IN ('open', 'in_review')").Count(&complaintPending)
	dash.ComplaintsPending = int(complaintPending)

	// Produk low-stock ecom untuk restock alert.
	var lowStockCount int64
	s.DB.Model(&entity.Product{}).
		Where("deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1 AND stock_ecom > 0 AND stock_ecom <= 5").
		Count(&lowStockCount)
	dash.LowStockCount = int(lowStockCount)

	return dash, nil
}

// GetLowStock — list produk stok tipis untuk widget alert. Threshold ≤5.
func (s *EcomAdminStatsService) GetLowStock(limit int) ([]dto.EcomLowStockItem, *dto.ApiError) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	var products []entity.Product
	if err := s.DB.
		Where("deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1 AND stock_ecom > 0 AND stock_ecom <= 5").
		Order("stock_ecom ASC").Limit(limit).Find(&products).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil low stock"}
	}
	out := make([]dto.EcomLowStockItem, 0, len(products))
	for _, p := range products {
		out = append(out, dto.EcomLowStockItem{
			ID:        p.ID,
			Name:      p.NameID,
			SKU:       p.SKU,
			StockEcom: p.StockEcom,
		})
	}
	return out, nil
}
