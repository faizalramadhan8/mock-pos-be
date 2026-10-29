package usecase

import (
	"strings"
	"sync"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"gorm.io/gorm"
)

// Tempat — dua sumbu yang beda dan sering tertukar:
//
//	orders.order_source   = SALURAN  : pos · ecom · home
//	products.location,
//	stock_movements.location,
//	bank_accounts.location,
//	ecom_settings.location = TEMPAT   : store · home
//
// Pemetaannya: pos→store, ecom→store, home→home. Dipisah karena storefront
// Bekasi nanti tetap bertempat `store` walaupun salurannya online.
const (
	TempatToko  = "store"
	TempatRumah = "home"
)

// TempatSah menormalkan masukan dari query param atau payload. Nilai asing
// jatuh ke rumah — itu yang dilayani storefront sejak awal, jadi salah ketik
// tidak pernah diam-diam menampilkan barang Bekasi.
func TempatSah(v string) string {
	if strings.TrimSpace(strings.ToLower(v)) == TempatToko {
		return TempatToko
	}
	return TempatRumah
}

// TempatDariOrderSource — saluran pesanan ke tempat asal barangnya.
// Dipakai saat memesan kurir: titik jemputnya harus toko yang benar.
func TempatDariOrderSource(src string) string {
	if strings.TrimSpace(strings.ToLower(src)) == "home" {
		return TempatRumah
	}
	return TempatToko
}

// Cache nama toko. Dipanggil per baris keranjang dan per kartu produk, jadi
// tanpa ini satu halaman katalog memicu puluhan query untuk dua nilai yang
// berubah beberapa kali setahun. TTL pendek supaya ganti nama di panel admin
// tetap terasa cepat tanpa perlu restart.
var (
	namaTokoMu    sync.RWMutex
	namaTokoCache = map[string]string{}
	namaTokoAt    = map[string]time.Time{}
)

const namaTokoTTL = 60 * time.Second

// NamaToko — nama yang customer lihat untuk sebuah tempat, diambil dari
// pengaturan toko itu sendiri. Kalau barisnya belum ada atau namanya kosong,
// pakai nama umum: layar tidak boleh menampilkan kode "home"/"store" mentah.
func NamaToko(db *gorm.DB, location string) string {
	loc := TempatSah(location)

	namaTokoMu.RLock()
	n, ok := namaTokoCache[loc]
	at := namaTokoAt[loc]
	namaTokoMu.RUnlock()
	if ok && time.Since(at) < namaTokoTTL {
		return n
	}

	nama := "Toko Rumah"
	if loc == TempatToko {
		nama = "Toko Bekasi"
	}
	if db != nil {
		var st entity.EcomSettings
		if err := db.Select("store_name").Where("location = ?", loc).First(&st).Error; err == nil {
			if v := strings.TrimSpace(st.StoreName); v != "" {
				nama = v
			}
		}
	}

	namaTokoMu.Lock()
	namaTokoCache[loc] = nama
	namaTokoAt[loc] = time.Now()
	namaTokoMu.Unlock()
	return nama
}

// LupakanNamaToko — buang cache setelah nama toko diubah di panel admin,
// supaya perubahannya langsung terlihat alih-alih menunggu TTL habis.
func LupakanNamaToko() {
	namaTokoMu.Lock()
	namaTokoCache = map[string]string{}
	namaTokoAt = map[string]time.Time{}
	namaTokoMu.Unlock()
}

// TempatProduk — rak tempat produk ini berada.
//
// Dipakai saat mencatat `stock_movements`: yang bergerak adalah barang di
// rak, jadi tempatnya ditentukan PRODUKNYA, bukan saluran pesanannya.
// Keduanya hampir selalu sama, tapi produk adalah satu-satunya yang tidak
// bisa salah — mencatat pengembalian stok Bekasi di rak rumah membuat dua
// audit stok meleset sekaligus, dan tidak ada yang memberi tahu.
func TempatProduk(db *gorm.DB, productID string) string {
	if db == nil || productID == "" {
		return TempatRumah
	}
	var p entity.Product
	if err := db.Select("location").Where("id = ?", productID).First(&p).Error; err != nil {
		return TempatRumah
	}
	return TempatSah(p.Location)
}

// SumberPesananTempat — nilai `orders.order_source` yang termasuk sebuah
// tempat. `location` kosong = dua-duanya (tanpa penyaring tempat).
//
// Dipakai penyaring "Toko Bekasi / Online Rumah / Gabungan" di panel admin.
// Jangan membandingkan `order_source` langsung dengan "ecom" — pesanan
// storefront Bekasi bersumber 'ecom' tapi bertempat `store`, dan pesanan
// toko rumah bersumber 'home'.
func SumberPesananTempat(location string) []string {
	switch strings.TrimSpace(strings.ToLower(location)) {
	case TempatToko:
		return []string{"ecom"}
	case TempatRumah:
		return []string{"home"}
	default:
		return []string{"ecom", "home"}
	}
}

// IsiInfoGrupCheckout — lengkapi detail pesanan dengan pesanan lain yang
// dibayar bersamaan.
//
// Keranjang dua toko menghasilkan dua pesanan dengan satu pembayaran. Tanpa
// keterangan ini, admin melihat dua tagihan untuk satu bukti transfer dan
// berisiko mengira uangnya kurang; kalau dikonfirmasi dua kali, mengira
// uangnya masuk dua kali. Customer pun perlu tahu nominal yang harus
// ditransfer adalah JUMLAH keduanya, bukan salah satunya.
//
// Best-effort: kegagalan query tidak membatalkan tampilan detail pesanan.
func IsiInfoGrupCheckout(db *gorm.DB, order *entity.Order, detail *dto.CustomerOrderDetail) {
	if db == nil || order == nil || detail == nil {
		return
	}
	if order.CheckoutGroupID == nil || strings.TrimSpace(*order.CheckoutGroupID) == "" {
		return
	}
	grup := *order.CheckoutGroupID
	var saudara []entity.Order
	if err := db.Select("id", "order_source", "total", "ecom_status").
		Where("checkout_group_id = ? AND deleted_at IS NULL", grup).
		Order("order_source ASC").Find(&saudara).Error; err != nil {
		return
	}
	// Grup berisi satu pesanan = checkout satu toko. Tidak perlu diberi
	// keterangan apa pun; menampilkannya justru menimbulkan pertanyaan.
	if len(saudara) < 2 {
		return
	}
	detail.CheckoutGroupID = grup
	for _, o := range saudara {
		st := ""
		if o.EcomStatus != nil {
			st = *o.EcomStatus
		}
		detail.GroupOrders = append(detail.GroupOrders, dto.OrderGroupSibling{
			OrderID:   o.ID,
			StoreName: NamaToko(db, TempatDariOrderSource(o.OrderSource)),
			Total:     o.Total,
			Status:    st,
			IsSelf:    o.ID == order.ID,
		})
		detail.GroupTotal += o.Total
	}
}

// TrxIDPembayaran — id transaksi yang dikenal payment gateway untuk sebuah
// pesanan.
//
// Sejak checkout lintas toko (30 Okt 2026) satu pembayaran bisa menutup
// beberapa pesanan, jadi yang dikirim ke PG adalah id GRUP. Pesanan lama
// tidak punya grup dan memakai id pesanannya sendiri.
//
// Memakai id pesanan untuk pesanan bergrup akan membuat PG menjawab "tidak
// ditemukan" — pengecekan status dan retry bayar diam-diam tidak berfungsi.
func TrxIDPembayaran(o *entity.Order) string {
	if o == nil {
		return ""
	}
	if o.CheckoutGroupID != nil && strings.TrimSpace(*o.CheckoutGroupID) != "" {
		return strings.TrimSpace(*o.CheckoutGroupID)
	}
	return o.ID
}

// Cache tempat-tempat yang tayang di storefront. Dibaca di tiap permintaan
// katalog publik, jadi tanpa cache satu halaman beranda memicu query
// tambahan untuk nilai yang berubah beberapa kali setahun.
var (
	tempatAktifMu sync.RWMutex
	tempatAktif   []string
	tempatAktifAt time.Time
)

// TempatTayang — daftar tempat yang produknya boleh muncul di storefront.
//
// Dipakai SEMUA jalur publik: daftar produk, kategori, detail produk,
// sitemap, tambah ke keranjang, dan checkout. Kalau salah satu terlewat,
// barang dari toko yang belum siap bisa dibeli lewat jalur itu — dan
// ongkirnya dihitung dari alamat jemput yang belum diisi.
//
// Gagal query → kembalikan rumah saja. Lebih baik separuh katalog hilang dan
// segera ketahuan daripada toko yang belum siap ikut tayang diam-diam.
func TempatTayang(db *gorm.DB) []string {
	tempatAktifMu.RLock()
	v, at := tempatAktif, tempatAktifAt
	tempatAktifMu.RUnlock()
	if v != nil && time.Since(at) < namaTokoTTL {
		return v
	}

	out := []string{}
	if db != nil {
		var rows []entity.EcomSettings
		if err := db.Select("location", "storefront_enabled").Find(&rows).Error; err == nil {
			for _, r := range rows {
				if r.StorefrontEnabled {
					out = append(out, TempatSah(r.Location))
				}
			}
		}
	}
	if len(out) == 0 {
		out = []string{TempatRumah}
	}

	tempatAktifMu.Lock()
	tempatAktif, tempatAktifAt = out, time.Now()
	tempatAktifMu.Unlock()
	return out
}

// TempatBolehTayang — apakah satu tempat sedang tayang di storefront.
func TempatBolehTayang(db *gorm.DB, location string) bool {
	loc := TempatSah(location)
	for _, v := range TempatTayang(db) {
		if v == loc {
			return true
		}
	}
	return false
}

// LupakanTempatTayang — buang cache setelah saklar diubah di panel admin.
func LupakanTempatTayang() {
	tempatAktifMu.Lock()
	tempatAktif, tempatAktifAt = nil, time.Time{}
	tempatAktifMu.Unlock()
}
