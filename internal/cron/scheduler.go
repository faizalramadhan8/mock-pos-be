package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/whatsapp"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type Scheduler struct {
	Log              *zerolog.Logger
	DB               *gorm.DB
	PushService      *usecase.PushService
	ProductRepo      *repository.ProductRepository
	BatchRepo        *repository.StockBatchRepository
	MoveRepo         *repository.StockMovementRepository
	PurchaseInvRepo  *repository.PurchaseInvoiceRepository
	AuthRepo         *repository.AuthRepository
	WA               *whatsapp.Service
}

func NewScheduler(ctx context.Context, db *gorm.DB) *Scheduler {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	wa, _ := ctx.Value(enum.WhatsAppCtxKey).(*whatsapp.Service)
	return &Scheduler{
		Log:             logger,
		DB:              db,
		PushService:     usecase.NewPushService(ctx, db),
		ProductRepo:     repository.NewProductRepository(ctx, db),
		BatchRepo:       repository.NewStockBatchRepository(ctx, db),
		MoveRepo:        repository.NewStockMovementRepository(ctx, db),
		PurchaseInvRepo: repository.NewPurchaseInvoiceRepository(ctx, db),
		AuthRepo:        repository.NewAuthRepository(ctx, db),
		WA:              wa,
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

	// H-0 Faktur Pembelian WA reminder. Daily 09:00 WIB. Query faktur
	// unpaid dengan due_date hari ini & belum dikirim reminder, broadcast
	// 1 ringkasan ke semua admin (Bu Santi confirm: sekali saja, no H+ retry).
	go func() {
		s.Log.Info().Msg("Purchase invoice H-0 reminder scheduler started")
		jkt := jktLocation()
		for {
			nowLocal := time.Now().In(jkt)
			next := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 9, 0, 0, 0, jkt)
			if nowLocal.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))
			s.runPurchaseInvoiceReminder()
		}
	}()
}

// jktLocation returns Asia/Jakarta tz (fallback UTC) — semua cron mengikuti
// jam toko Bu Santi, bukan jam server.
func jktLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.UTC
	}
	return loc
}

// runPurchaseInvoiceReminder kirim WA H-0 ke admin untuk faktur jatuh tempo
// hari ini. 1× per faktur (tracked via reminder_sent_at). Bu Santi:
// "cukup sekali saja, dan dikirim ke nomor yang sama seperti yang dapat
// notifikasi transaksi baru" — same source: AuthRepo.FindAdmins().
func (s *Scheduler) runPurchaseInvoiceReminder() {
	if s.WA == nil || !s.WA.Enabled() {
		s.Log.Info().Msg("Purchase invoice reminder: WA disabled, skip")
		return
	}

	today := time.Now().In(jktLocation())
	invoices, err := s.PurchaseInvRepo.FindDueToday(today)
	if err != nil {
		s.Log.Error().Err(err).Msg("Purchase invoice reminder: query failed")
		return
	}
	if len(invoices) == 0 {
		s.Log.Info().Msg("Purchase invoice reminder: no due-today invoices")
		return
	}

	admins, err := s.AuthRepo.FindAdmins()
	if err != nil || len(admins) == 0 {
		s.Log.Warn().Err(err).Msg("Purchase invoice reminder: no admin recipients")
		return
	}

	text := formatPurchaseInvoiceReminder(invoices, today)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	for _, a := range admins {
		if err := s.WA.SendText(ctx, a.PhoneNumber, text); err != nil {
			s.Log.Warn().Err(err).Str("admin_id", a.ID).Msg("Purchase invoice reminder WA send failed")
		}
	}

	// Mark reminder_sent_at supaya tidak double-send next run.
	now := time.Now()
	for _, inv := range invoices {
		if err := s.PurchaseInvRepo.MarkReminderSent(s.DB, inv.ID, now); err != nil {
			s.Log.Warn().Err(err).Str("invoice_id", inv.ID).Msg("Failed to mark reminder_sent_at")
		}
	}
}

// formatPurchaseInvoiceReminder — ringkasan list faktur due today untuk admin.
// Format mirror admin notif transaksi besar supaya konsisten visual style.
func formatPurchaseInvoiceReminder(invoices []entity.PurchaseInvoice, today time.Time) string {
	var b strings.Builder
	jkt := jktLocation()

	fmt.Fprintf(&b, "*🧾 FAKTUR JATUH TEMPO HARI INI*\n")
	fmt.Fprintf(&b, "_Toko Bahan Kue Santi_\n\n")
	fmt.Fprintf(&b, "Tanggal: %s\n", today.In(jkt).Format("02 Jan 2006"))
	fmt.Fprintf(&b, "Total: %d faktur\n\n", len(invoices))

	for i, inv := range invoices {
		supplierName := "-"
		if inv.Supplier != nil {
			supplierName = inv.Supplier.Name
		}
		fmt.Fprintf(&b, "%d. *%s*\n", i+1, supplierName)
		if inv.InvoiceNumber != "" {
			fmt.Fprintf(&b, "   No. %s\n", inv.InvoiceNumber)
		}
		fmt.Fprintf(&b, "   Total: *%s*\n", formatRupiahCron(inv.TotalAmount))
		fmt.Fprintf(&b, "   Tempo: %s\n", inv.PaymentTerms)
		if i < len(invoices)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// formatRupiahCron — local copy untuk hindari cross-package coupling.
func formatRupiahCron(v float64) string {
	n := int64(v + 0.5)
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	if neg {
		return "Rp -" + string(out)
	}
	return "Rp " + string(out)
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

	title := "Toko Bahan Kue Santi — Alert"
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
