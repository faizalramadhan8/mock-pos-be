package usecase

import (
	"math"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

// Harga grosir di storefront — mode PAKET, sama persis dengan kasir.
//
// Sampai 18 Okt 2026 checkout menagih `ecom_price × qty` begitu saja dan tidak
// pernah melihat `product_price_tiers`. Padahal halaman produk SUDAH
// menampilkan tabel "Harga Grosir", menandainya "aktif" begitu qty cukup, dan
// tombolnya berbunyi "Beli · Rp <harga grosir × qty>". Jadi customer dikutip
// harga grosir lalu ditagih harga normal — persis di fitur yang jadi inti
// model bisnis toko rumah Bu Santi ("menyasar pembelian kartonan").
//
// Aturannya mengikuti keputusan Bu Santi 21 Jun 2026: tier berlaku HANYA dalam
// kelipatan min_qty, sisanya harga biasa. "Beli 2 dapat 33rb" berarti 2 = 33rb,
// 3 = 33rb + 1 satuan, 4 = 66rb. Bukan "sekali lewat ambang semua unit murah" —
// itu versi lama yang dia tolak karena terlalu murah hati dan tidak cocok
// dengan cara orang berdagang di sini.
type ecomLinePrice struct {
	UnitPrice   float64 // LineTotal / qty — supaya unit_price × quantity tetap = LineTotal
	LineTotal   float64
	PriceSource string // "regular" | "tier_all"
	TierID      *string
	PaketCount  int
	ExtraCount  int
}

// ecomBasePrice — harga satuan sebelum tier. Storefront memakai ecom_price
// kalau diisi, kalau tidak jatuh ke harga jual kasir.
func ecomBasePrice(p *entity.Product) float64 {
	if p.EcomPrice != nil {
		return *p.EcomPrice
	}
	return p.SellingPrice
}

// computeEcomLinePrice — hitung harga satu baris keranjang.
//
// Hanya tier `all_customers` yang dipakai: storefront tidak punya konsep member
// POS, jadi tier `member_specific` tidak boleh bocor ke publik. Tier kedaluwarsa
// dilewati, sama seperti `computeBestUnitPrice` di kasir.
//
// Kegagalan membaca tier TIDAK membatalkan checkout — jatuh ke harga normal.
// Checkout yang gagal karena tabel tier bermasalah jauh lebih merugikan
// daripada satu pesanan yang kehilangan diskon grosirnya.
func computeEcomLinePrice(db *gorm.DB, p *entity.Product, qty int) ecomLinePrice {
	base := ecomBasePrice(p)
	plain := ecomLinePrice{
		UnitPrice:   base,
		LineTotal:   math.Round(base * float64(qty)),
		PriceSource: "regular",
	}
	if qty <= 0 {
		return plain
	}

	var tiers []entity.ProductPriceTier
	if err := db.Where(
		"product_id = ? AND deleted_at IS NULL AND target_type = 'all_customers'", p.ID,
	).Find(&tiers).Error; err != nil {
		return plain
	}

	// Greedy: min_qty terbesar yang muat. Kalau seri, harga terendah menang.
	now := time.Now()
	var best *entity.ProductPriceTier
	for i := range tiers {
		t := &tiers[i]
		if t.MinQty <= 0 || qty < t.MinQty {
			continue
		}
		if t.ExpiresAt != nil && t.ExpiresAt.Before(now) {
			continue
		}
		if best == nil || t.MinQty > best.MinQty || (t.MinQty == best.MinQty && t.Price < best.Price) {
			best = t
		}
	}
	// Tier yang lebih mahal dari harga normal diabaikan — grosir tidak boleh
	// membuat customer membayar lebih banyak karena membeli lebih banyak.
	if best == nil || best.Price >= base {
		return plain
	}

	paket := qty / best.MinQty
	extra := qty % best.MinQty
	// Dibulatkan ke rupiah utuh. Harga tier tersimpan DECIMAL(15,2) hasil bagi
	// (mis. "beli 3 = 29.000" → 9.666,67), jadi tanpa pembulatan totalnya
	// meleset satu sen — persis keluhan Bu Santi 21 Jun 2026.
	lineTotal := math.Round(float64(paket*best.MinQty)*best.Price + float64(extra)*base)
	tierID := best.ID
	return ecomLinePrice{
		UnitPrice:   lineTotal / float64(qty),
		LineTotal:   lineTotal,
		PriceSource: "tier_all",
		TierID:      &tierID,
		PaketCount:  paket,
		ExtraCount:  extra,
	}
}
