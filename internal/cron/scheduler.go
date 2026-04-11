package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type Scheduler struct {
	Log         *zerolog.Logger
	DB          *gorm.DB
	PushService *usecase.PushService
	ProductRepo *repository.ProductRepository
	BatchRepo   *repository.StockBatchRepository
	MoveRepo    *repository.StockMovementRepository
}

func NewScheduler(ctx context.Context, db *gorm.DB) *Scheduler {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &Scheduler{
		Log:         logger,
		DB:          db,
		PushService: usecase.NewPushService(ctx, db),
		ProductRepo: repository.NewProductRepository(ctx, db),
		BatchRepo:   repository.NewStockBatchRepository(ctx, db),
		MoveRepo:    repository.NewStockMovementRepository(ctx, db),
	}
}

func (s *Scheduler) Start() {
	go func() {
		s.Log.Info().Msg("Push notification scheduler started")
		for {
			now := time.Now()
			// Schedule for 07:00 every day
			next := time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))
			s.checkAndNotify()
		}
	}()
}

func (s *Scheduler) checkAndNotify() {
	s.Log.Info().Msg("Running daily notification check")

	var alerts []string

	// Check low stock
	lowStock, err := s.ProductRepo.FindLowStock()
	if err == nil && len(lowStock) > 0 {
		alerts = append(alerts, fmt.Sprintf("%d produk stok rendah/habis", len(lowStock)))
	}

	// Check expiring batches (within 7 days)
	expiring, err := s.BatchRepo.FindExpiring(7)
	if err == nil && len(expiring) > 0 {
		alerts = append(alerts, fmt.Sprintf("%d batch segera kadaluarsa", len(expiring)))
	}

	// Check overdue invoices
	movements, _, err := s.MoveRepo.FindAll("", 500, 0)
	if err == nil {
		overdue := 0
		now := time.Now()
		for _, m := range movements {
			if m.PaymentStatus == "unpaid" && m.DueDate != nil {
				dueDate, err := time.Parse("2006-01-02", *m.DueDate)
				if err == nil && dueDate.Before(now) {
					overdue++
				}
			}
		}
		if overdue > 0 {
			alerts = append(alerts, fmt.Sprintf("%d invoice jatuh tempo", overdue))
		}
	}

	if len(alerts) == 0 {
		s.Log.Info().Msg("No alerts to send")
		return
	}

	title := "BakeShop Alert"
	body := ""
	for i, a := range alerts {
		if i > 0 {
			body += ", "
		}
		body += a
	}

	s.Log.Info().Str("body", body).Msg("Sending push notifications")
	s.PushService.SendToAll(title, body, "/")
}
