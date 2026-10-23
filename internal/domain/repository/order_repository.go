package repository

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

type OrderRepository struct {
	DB *gorm.DB
}

func NewOrderRepository(ctx context.Context, db *gorm.DB) *OrderRepository {
	return &OrderRepository{DB: db}
}

// FindAll — daftar order dengan filter status + sumber.
//
// Riwayat: 29 Jul 2026 ecom sengaja di-exclude dari POS ("cegah
// pending_payment ecom masuk Total Pendapatan"). Bu Santi 17 Sep 2026 minta
// kebalikannya — digabung dengan penanda: "Paling nanti di kasih tanda aja
// ya. Antara e commerce dan offline."
//
// Aman digabung karena `orders.status` konsisten lintas sumber: ecom yang
// dibayar ikut di-set `completed` (lihat ecom_checkout_usecase PGWebhook),
// yang belum bayar tetap `pending`. Jadi agregat omzet yang memfilter
// `status='completed'` tidak akan kemasukan pending_payment ecom.
//
// source: "" / "all" = semua, "pos" = toko, "ecom" = online.
func (r *OrderRepository) FindAll(status, source string, limit, offset int) ([]entity.Order, int64, error) {
	var orders []entity.Order
	var total int64

	query := r.DB.Model(&entity.Order{})
	switch source {
	case "pos":
		// Order lama (sebelum kolom order_source ada) dianggap POS.
		query = query.Where("(order_source = 'pos' OR order_source IS NULL OR order_source = '')")
	case "ecom":
		query = query.Where("order_source = 'ecom'")
	case "home":
		// Toko rumah — unit usaha terpisah dari Bekasi.
		query = query.Where("order_source = 'home'")
	case "store":
		// Semua yang terjadi DI Bekasi: kasir + (nanti) storefront Bekasi.
		// Ini yang dipakai Bu Santi saat closing — uang rumah masuk rekening
		// lain dan tidak pernah lewat laci kasir Bekasi.
		query = query.Where("(order_source IN ('pos','ecom') OR order_source IS NULL OR order_source = '')")
	}
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	query.Count(&total)

	if err := query.Preload("Items").Preload("Payments").Preload("Member").Order("created_at DESC").Limit(limit).Offset(offset).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *OrderRepository) FindByID(id string) (*entity.Order, error) {
	var order entity.Order
	if err := r.DB.Preload("Items").Preload("Payments").Preload("Member").Where("id = ?", id).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) FindByDateRange(startDate, endDate string) ([]entity.Order, error) {
	var orders []entity.Order
	if err := r.DB.Preload("Items").Preload("Payments").Preload("Member").Where("DATE(created_at) BETWEEN ? AND ?", startDate, endDate).Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// FindRecentByUserAndTotal returns completed orders dari kasir tertentu
// dengan total exact match, created dalam window time terakhir. Dipakai
// defense layer #2 dedup: kalau ada candidate yang items signature-nya
// match → skenario refresh-then-resubmit. Limit 5 candidates untuk capping
// query cost. Preload Items supaya caller bisa compare signature.
func (r *OrderRepository) FindRecentByUserAndTotal(userID string, total float64, since time.Time) ([]entity.Order, error) {
	var orders []entity.Order
	if err := r.DB.Preload("Items").
		Where("created_by = ? AND total = ? AND status = ? AND created_at >= ?", userID, total, "completed", since).
		Order("created_at DESC").
		Limit(5).
		Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// FindCompletedSince returns completed orders created after a given timestamp.
// Used by the cron-based admin transaction digest (kirim ringkasan tiap 30
// menit) supaya tidak fire WA per-checkout — cegah ban risk dari volume tinggi.
func (r *OrderRepository) FindCompletedSince(since time.Time) ([]entity.Order, error) {
	var orders []entity.Order
	if err := r.DB.Preload("Items").Preload("Payments").Preload("Member").
		Where("status = ? AND created_at > ?", "completed", since).
		Order("created_at ASC").Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

// FindCompletedForAggregate — fetch all completed orders in date range with
// preloaded items + payments + member. Dipakai oleh /orders/aggregate
// endpoint untuk hitung statistik server-side (cegah FE load full orders
// untuk Reports + Dashboard yang scale buruk pada volume tinggi).
//
// from/to: YYYY-MM-DD string. Empty = no bound. Range inclusive both sides.
// FindCompletedForAggregate — pesanan lunas untuk laporan.
//
// `source` memisahkan tempat usaha: "store" = Bekasi (kasir + nanti
// storefront Bekasi), "home" = toko rumah, kosong = gabungan. Bu Santi
// 18 Okt 2026 minta keduanya terpisah karena uang toko rumah masuk rekening
// sendiri dan tidak pernah lewat laci kasir Bekasi — kalau digabung,
// angkanya tidak akan pernah cocok saat closing.
func (r *OrderRepository) FindCompletedForAggregate(from, to, source string) ([]entity.Order, error) {
	var orders []entity.Order
	q := r.DB.Model(&entity.Order{}).Where("status = ?", "completed")
	switch source {
	case "home":
		q = q.Where("order_source = 'home'")
	case "store":
		q = q.Where("(order_source IN ('pos','ecom') OR order_source IS NULL OR order_source = '')")
	}
	if from != "" {
		q = q.Where("DATE(created_at) >= ?", from)
	}
	if to != "" {
		q = q.Where("DATE(created_at) <= ?", to)
	}
	if err := q.Preload("Items").Preload("Payments").Preload("Member").
		Order("created_at ASC").Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *OrderRepository) Create(order *entity.Order) error {
	return r.DB.Create(order).Error
}

func (r *OrderRepository) Update(order *entity.Order) error {
	// Nil-out preloaded Member (BelongsTo). Lihat product_repository.go untuk
	// detail bug GORM Save + Preload. Items dan Payments HasMany — handled
	// terpisah oleh GORM.
	order.Member = nil
	return r.DB.Save(order).Error
}

// FindByMember returns orders for a member within optional date range.
// from/to are inclusive YYYY-MM-DD strings; pass "" to skip bound.
func (r *OrderRepository) FindByMember(memberID, from, to string, limit, offset int) ([]entity.Order, int64, error) {
	var orders []entity.Order
	var total int64

	q := r.DB.Model(&entity.Order{}).Where("member_id = ? AND status = 'completed'", memberID)
	if from != "" {
		q = q.Where("DATE(created_at) >= ?", from)
	}
	if to != "" {
		q = q.Where("DATE(created_at) <= ?", to)
	}
	q.Count(&total)

	if limit <= 0 {
		limit = 1000
	}
	if err := q.Preload("Items").Order("created_at DESC").Limit(limit).Offset(offset).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (r *OrderRepository) GetRevenueStats() (float64, int64, error) {
	var result struct {
		Total float64
		Count int64
	}
	err := r.DB.Model(&entity.Order{}).
		Where("status = 'completed'").
		Select("COALESCE(SUM(total), 0) as total, COUNT(*) as count").
		Scan(&result).Error
	return result.Total, result.Count, err
}
