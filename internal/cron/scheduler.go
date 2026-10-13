package cron

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	MemberRepo       *repository.MemberRepository
	PointMoveRepo    *repository.MemberPointMovementRepository
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
		MemberRepo:      repository.NewMemberRepository(ctx, db),
		PointMoveRepo:   repository.NewMemberPointMovementRepository(ctx, db),
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

	// Member points expiry — reset semua saldo ke 0 setiap 1 Januari 00:01
	// WIB. Audit movements 'expire-reset' di-insert per member yang punya
	// saldo > 0 supaya kapan & berapa hangus tetap bisa ditrace.
	go func() {
		s.Log.Info().Msg("Member points expiry scheduler started")
		jkt := jktLocation()
		for {
			nowLocal := time.Now().In(jkt)
			next := time.Date(nowLocal.Year()+1, time.January, 1, 0, 1, 0, 0, jkt)
			time.Sleep(time.Until(next))
			s.runMemberPointsExpiry()
		}
	}()

	// Ecom auto-complete — kalau customer lupa konfirmasi "Barang Diterima"
	// setelah 7 hari sejak kurir tag delivered, sistem auto-mark completed.
	// Sesuai pattern Tokopedia/Shopee. Cron jalan tiap hari 03:00 WIB
	// (low-traffic supaya tidak ganggu jam operasional toko).
	go func() {
		s.Log.Info().Msg("Ecom auto-complete scheduler started")
		jkt := jktLocation()
		for {
			nowLocal := time.Now().In(jkt)
			next := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 3, 0, 0, 0, jkt)
			if nowLocal.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))
			s.runEcomAutoComplete()
		}
	}()

	// Ecom expire checkout terbengkalai. Stok di-reserve saat customer klik
	// bayar, tapi kalau dia tidak pernah menyelesaikan pembayaran dan PG tidak
	// pernah mengirim webhook FAILED/VOIDED, reservasi itu menggantung selamanya.
	// Sejak stok toko dan online digabung (migration 000073) yang tertahan
	// adalah stok fisik — kasir tidak bisa menjualnya padahal barangnya ada di
	// rak. Jalan tiap jam supaya stok cepat kembali, bukan harian.
	go func() {
		s.Log.Info().Msg("Ecom expire-unpaid scheduler started")
		for {
			time.Sleep(time.Hour)
			s.runEcomExpireUnpaid()
		}
	}()
}

// ecomUnpaidTTL — batas sabar menunggu pembayaran online. 24 jam sama dengan
// masa berlaku link pembayaran DOKU, jadi order yang lewat ini sudah pasti
// tidak bisa dibayar lagi lewat link yang sama.
//
// PENTING: angka ini berasal dari masa berlaku link payment gateway. Untuk
// transfer manual tidak ada link yang kedaluwarsa — yang ditunggu adalah
// Bu Santi memeriksa mutasi rekening, dan itu bisa lewat akhir pekan. Karena
// itu pesanan yang SUDAH mengunggah bukti transfer dikecualikan (lihat query
// di runEcomExpireUnpaid), bukan diberi TTL lebih panjang: yang menentukan
// bukan lamanya waktu, melainkan apakah customer sudah membayar.
const ecomUnpaidTTL = 24 * time.Hour

// runEcomExpireUnpaid — tandai `expired` semua ecom order yang masih
// pending_payment lewat dari ecomUnpaidTTL, kembalikan stoknya, dan catat
// movement-nya. Per order satu transaksi supaya satu order bermasalah tidak
// memblokir sisanya.
func (s *Scheduler) runEcomExpireUnpaid() {
	cutoff := time.Now().Add(-ecomUnpaidTTL)

	var orders []entity.Order
	if err := s.DB.Preload("Items").
		// Bukti transfer sudah ada = customer SUDAH bayar, tinggal menunggu
		// Bu Santi mencocokkan mutasi. Membatalkannya berarti: uang sudah
		// masuk ke rekening, stok dikembalikan, pesanan ditandai kedaluwarsa,
		// dan customer tidak menerima apa pun. Kerugian uang sekaligus
		// kepercayaan — dan korbannya justru customer yang paling patuh.
		//
		// Skenario nyatanya sederhana: transfer Sabtu malam, Bu Santi buka
		// rekening Senin. Cron jalan tiap jam, jadi pesanannya sudah hangus
		// jauh sebelum sempat dilihat.
		Where(`order_source = 'ecom' AND ecom_status = 'pending_payment'
		       AND created_at <= ? AND deleted_at IS NULL
		       AND (payment_proof IS NULL OR payment_proof = '')`, cutoff).
		Limit(200).Find(&orders).Error; err != nil {
		s.Log.Error().Err(err).Msg("Ecom expire-unpaid: query failed")
		return
	}
	if len(orders) == 0 {
		return
	}

	expired := 0
	for i := range orders {
		o := orders[i]
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			// Re-check di dalam tx — webhook PG bisa saja baru masuk dan
			// menandai order ini paid tepat sebelum kita proses.
			var fresh entity.Order
			if e := tx.Where("id = ?", o.ID).First(&fresh).Error; e != nil {
				return e
			}
			if fresh.EcomStatus == nil || *fresh.EcomStatus != "pending_payment" {
				return nil // sudah berubah, jangan sentuh
			}
			// Bukti bisa masuk di sela antara query dan transaksi ini.
			if strings.TrimSpace(fresh.PaymentProof) != "" {
				return nil
			}

			for _, it := range o.Items {
				if it.ProductID == "" {
					continue
				}
				if e := tx.Model(&entity.Product{}).Where("id = ?", it.ProductID).
					Update("stock", gorm.Expr("stock + ?", it.Quantity)).Error; e != nil {
					return e
				}
				if e := tx.Create(&entity.StockMovement{
					ID:        uuid.New().String(),
					ProductID: it.ProductID,
					Type:      "in",
					Quantity:  it.Quantity,
					UnitType:  "individual",
					UnitPrice: it.UnitPrice,
					Reason:    "cancel",
					Note:      fmt.Sprintf("Pesanan online kedaluwarsa, tidak dibayar (Order #%s)", o.ID[:8]),
					CreatedBy: o.CreatedBy,
				}).Error; e != nil {
					return e
				}
			}
			return tx.Model(&entity.Order{}).Where("id = ?", o.ID).
				Updates(map[string]interface{}{
					"ecom_status": "expired",
					"status":      "cancelled",
				}).Error
		})
		if err != nil {
			s.Log.Error().Err(err).Str("order_id", o.ID).Msg("Ecom expire-unpaid: order failed")
			continue
		}
		expired++
	}
	s.Log.Info().Int("expired", expired).Int("scanned", len(orders)).Msg("Ecom expire-unpaid: done")
}

// runEcomAutoComplete — mark completed semua ecom order yang ecom_status =
// 'delivered' AND ecom_delivered_at <= NOW - 7 hari. Fallback kalau customer
// lupa konfirmasi. Batch single UPDATE — cukup untuk skala Bu Santi (belasan
// order/hari maks).
func (s *Scheduler) runEcomAutoComplete() {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	res := s.DB.Exec(`
		UPDATE orders
		SET ecom_status = 'completed', status = 'completed'
		WHERE order_source = 'ecom'
		  AND ecom_status = 'delivered'
		  AND ecom_delivered_at IS NOT NULL
		  AND ecom_delivered_at <= ?
		  AND deleted_at IS NULL
	`, cutoff)
	if res.Error != nil {
		s.Log.Error().Err(res.Error).Msg("Ecom auto-complete: update failed")
		return
	}
	s.Log.Info().Int64("affected", res.RowsAffected).Msg("Ecom auto-complete: done")
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

// runMemberPointsExpiry — set semua member.points = 0 dan insert audit
// movement 'expire-reset' per member yang punya saldo > 0 sebelum reset.
// Per Bu Santi: poin hangus 1 Januari. Audit trail penting supaya kalau
// member tanya "kenapa saya nol", kasir bisa tunjuk movement-nya.
func (s *Scheduler) runMemberPointsExpiry() {
	members, err := s.MemberRepo.FindAllWithPositivePoints()
	if err != nil {
		s.Log.Error().Err(err).Msg("Member points expiry: query members failed")
		return
	}
	if len(members) == 0 {
		s.Log.Info().Msg("Member points expiry: tidak ada member dengan saldo > 0")
		return
	}

	now := time.Now()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		for _, m := range members {
			if err := tx.Create(&entity.MemberPointMovement{
				ID:           uuid.New().String(),
				MemberID:     m.ID,
				Type:         "expire-reset",
				Points:       -m.Points,
				BalanceAfter: 0,
				Note:         "Reset tahunan 1 Januari",
				CreatedAt:    now,
			}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&entity.Member{}).Where("points > 0").Update("points", 0).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		s.Log.Error().Err(err).Msg("Member points expiry: transaction failed")
		return
	}
	s.Log.Info().Int("members_reset", len(members)).Msg("Member points expiry: done")
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

	// Faktur lewat tempo — baca purchase_invoices, BUKAN stock_movements.
	//
	// Sampai 29 Sep 2026 blok ini menghitung dari `stock_movements.due_date`
	// (skema faktur lama). Alur faktur sekarang tidak menulis movement sama
	// sekali, jadi angka ini selalu 0 dan digest tidak pernah menyebut faktur.
	// Batas bawah sengaja tidak dipasang — tunggakan lama tetap dihitung.
	today := time.Now().Format("2006-01-02")
	if invoices, _, err := s.PurchaseInvRepo.FindAll(repository.InvoiceFilter{
		Status:    "unpaid",
		To:        today,
		DateField: "due",
		Limit:     500,
	}); err == nil && len(invoices) > 0 {
		var totalDue float64
		for _, inv := range invoices {
			totalDue += inv.TotalAmount
		}
		alerts = append(alerts, fmt.Sprintf("%d faktur jatuh tempo (total Rp %.0f)", len(invoices), totalDue))
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
