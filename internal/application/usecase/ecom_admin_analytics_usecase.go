package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// EcomAdminAnalyticsService — Sprint 5 Chunk 8 (2 Aug 2026).
// Aggregate metrics untuk analitik mendalam admin panel ecom (trend + funnel
// + payment split + AOV). Query cepat via GROUP BY, tidak load full rows.
type EcomAdminAnalyticsService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAdminAnalyticsService(ctx context.Context, db *gorm.DB) *EcomAdminAnalyticsService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAdminAnalyticsService{DB: db, Log: logger}
}

// Get — semua metrik dalam 1 request, cegah round-trip. Range = 30 hari default.
func (s *EcomAdminAnalyticsService) Get() (*dto.EcomAdminAnalytics, *dto.ApiError) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	// Range 30 hari — from 00:00 hari ke-30 lalu, to 23:59 hari ini.
	from := now.AddDate(0, 0, -29).Truncate(24 * time.Hour)
	to := now.Add(24 * time.Hour).Truncate(24 * time.Hour)

	resp := &dto.EcomAdminAnalytics{
		RangeFrom: from.Format("2006-01-02"),
		RangeTo:   now.Format("2006-01-02"),
	}

	// Daily revenue (completed only, agar konsisten sebagai actual cash).
	// Prealloc 30-slot template untuk hari yang tidak ada order (0 revenue).
	daily := make([]dto.AnalyticsDailyBucket, 30)
	for i := 0; i < 30; i++ {
		d := from.AddDate(0, 0, i)
		daily[i] = dto.AnalyticsDailyBucket{
			Date:    d.Format("2006-01-02"),
			Revenue: 0,
			Orders:  0,
		}
	}
	dailyRows := []struct {
		Day     string
		Revenue float64
		Orders  int
	}{}
	s.DB.Table("orders").
		Select("DATE_FORMAT(created_at, '%Y-%m-%d') AS day, SUM(total) AS revenue, COUNT(*) AS orders").
		Where("order_source = 'ecom' AND deleted_at IS NULL AND status = 'completed' AND created_at >= ? AND created_at < ?", from, to).
		Group("day").
		Scan(&dailyRows)
	dayIdx := make(map[string]int, 30)
	for i, b := range daily {
		dayIdx[b.Date] = i
	}
	for _, r := range dailyRows {
		if idx, ok := dayIdx[r.Day]; ok {
			daily[idx].Revenue = r.Revenue
			daily[idx].Orders = r.Orders
		}
	}
	resp.DailyRevenue = daily

	// Funnel — pakai ecom_status supaya track full journey (bukan status utama order).
	// Count semua orders (bukan cuma completed) dalam periode.
	type funnelRow struct {
		Status string
		Count  int
	}
	funnelRows := []funnelRow{}
	s.DB.Table("orders").
		Select("COALESCE(ecom_status, status) AS status, COUNT(*) AS count").
		Where("order_source = 'ecom' AND deleted_at IS NULL AND created_at >= ? AND created_at < ?", from, to).
		Group("status").Scan(&funnelRows)
	funnel := map[string]int{}
	for _, r := range funnelRows {
		funnel[r.Status] = r.Count
	}
	resp.Funnel = funnel

	// KPI aggregate
	kpi := struct {
		TotalOrders    int
		CompletedCount int
		TotalRevenue   float64
		AvgOrderValue  float64
	}{}
	s.DB.Table("orders").
		Where("order_source = 'ecom' AND deleted_at IS NULL AND created_at >= ? AND created_at < ?", from, to).
		Select(`COUNT(*) AS total_orders,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) AS completed_count,
			COALESCE(SUM(CASE WHEN status = 'completed' THEN total ELSE 0 END), 0) AS total_revenue,
			COALESCE(AVG(CASE WHEN status = 'completed' THEN total ELSE NULL END), 0) AS avg_order_value`).
		Scan(&kpi)
	resp.TotalOrders = kpi.TotalOrders
	resp.CompletedCount = kpi.CompletedCount
	resp.TotalRevenue = kpi.TotalRevenue
	resp.AvgOrderValue = kpi.AvgOrderValue
	if kpi.TotalOrders > 0 {
		resp.ConversionRate = float64(kpi.CompletedCount) / float64(kpi.TotalOrders) * 100
	}
	// Cancel rate — indikator quality (customer batal krn stock/harga/etc).
	if kpi.TotalOrders > 0 {
		cancelled := funnel["cancelled"]
		resp.CancelRate = float64(cancelled) / float64(kpi.TotalOrders) * 100
	}

	// Payment channel split (dari order_payments, hanya completed orders).
	type channelRow struct {
		Method string
		Count  int
		Amount float64
	}
	channelRows := []channelRow{}
	s.DB.Table("order_payments op").
		Joins("JOIN orders o ON o.id = op.order_id").
		Select("op.method AS method, COUNT(*) AS count, COALESCE(SUM(op.amount), 0) AS amount").
		Where("o.order_source = 'ecom' AND o.deleted_at IS NULL AND o.status = 'completed' AND o.created_at >= ? AND o.created_at < ?", from, to).
		Group("op.method").Scan(&channelRows)
	channels := make([]dto.AnalyticsChannelSlice, 0, len(channelRows))
	for _, r := range channelRows {
		channels = append(channels, dto.AnalyticsChannelSlice{
			Method: r.Method,
			Count:  r.Count,
			Amount: r.Amount,
		})
	}
	resp.PaymentChannels = channels

	return resp, nil
}

// ptr helper for optional error field
var _ = fiber.StatusOK
