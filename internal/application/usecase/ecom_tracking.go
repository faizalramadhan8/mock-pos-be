package usecase

import (
	"strings"
	"sync"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/gofiber/fiber/v2"
)

// Pelacakan paket untuk customer — riwayat perjalanan dari Biteship.
//
// Sebelum ini satu-satunya cara customer melihat posisi paketnya adalah tombol
// "Lacak Paket" yang MELEMPAR KELUAR ke track.biteship.com. Keluar dari toko
// untuk tahu barangnya sampai mana adalah tempat paling mudah kehilangan orang.

// trackingTTL — Biteship punya batas laju, dan customer cenderung menarik-
// refresh berkali-kali saat menunggu paket. Data pelacakan juga bergerak dalam
// hitungan jam, bukan detik, jadi menahannya sebentar tidak mengurangi
// ketepatan sedikit pun.
const trackingTTL = 60 * time.Second

type trackingCacheEntry struct {
	data *dto.OrderTrackingResponse
	at   time.Time
}

var (
	trackingCacheMu sync.Mutex
	trackingCache   = map[string]trackingCacheEntry{}
)

// statusIndo — Biteship mengirim catatan berbahasa Inggris ("Courier order is
// confirmed. jnt has been notified to pick up"). Customer Bu Santi tidak
// membaca itu. Catatan aslinya tetap dibawa sebagai cadangan supaya status
// yang belum dipetakan tidak pernah tampil kosong.
var statusIndo = map[string]string{
	"confirmed":         "Pesanan diteruskan ke kurir",
	"allocated":         "Kurir sudah ditugaskan",
	"picking_up":        "Kurir menuju lokasi penjemputan",
	"picked":            "Paket sudah dijemput kurir",
	"dropping_off":      "Paket dalam perjalanan ke alamat tujuan",
	"delivered":         "Paket sampai di tujuan",
	"on_hold":           "Paket tertahan sementara",
	"courier_not_found": "Belum ada kurir yang mengambil",
	"rejected":          "Pengiriman ditolak kurir",
	"cancelled":         "Pengiriman dibatalkan",
	"return_in_transit": "Paket sedang dikembalikan ke pengirim",
	"returned":          "Paket sudah kembali ke pengirim",
	"disposed":          "Paket dimusnahkan kurir",
}

// GetTracking — riwayat perjalanan paket milik customer sendiri.
//
// Mengembalikan `nil, nil` kalau resinya belum terbit. Itu keadaan normal
// (nomor resi baru turun lewat webhook setelah kurir mengalokasikan), bukan
// kesalahan — FE menampilkannya sebagai "Resi belum terbit", bukan layar error.
func (s *EcomOrdersService) GetTracking(userID, orderID string) (*dto.OrderTrackingResponse, *dto.ApiError) {
	var order entity.Order
	if err := s.DB.Where(
		"id = ? AND ecom_user_id = ? AND order_source = 'ecom' AND deleted_at IS NULL", orderID, userID,
	).First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Pesanan tidak ditemukan"}
	}

	trackingID := ""
	if order.ShippingTrackingID != nil {
		trackingID = strings.TrimSpace(*order.ShippingTrackingID)
	}
	if trackingID == "" {
		return nil, nil // resi belum terbit
	}

	trackingCacheMu.Lock()
	if e, ok := trackingCache[trackingID]; ok && time.Since(e.at) < trackingTTL {
		trackingCacheMu.Unlock()
		return e.data, nil
	}
	trackingCacheMu.Unlock()

	t, err := s.Admin.shippingClient().GetTracking(trackingID)
	if err != nil {
		s.Log.Warn().Err(err).Str("order_id", orderID).Msg("gagal ambil tracking Biteship")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrBadGateway,
			Message:    "Gagal mengambil status pengiriman. Coba lagi sebentar lagi.",
		}
	}

	resp := &dto.OrderTrackingResponse{
		WaybillID: t.WaybillID,
		Courier:   t.Courier,
		Status:    t.Status,
		Link:      t.Link,
	}
	for _, h := range t.History {
		label := statusIndo[strings.ToLower(strings.TrimSpace(h.Status))]
		if label == "" {
			label = h.Note // status baru dari Biteship — tampilkan apa adanya
		}
		resp.History = append(resp.History, dto.OrderTrackingEvent{
			Status:    h.Status,
			Label:     label,
			Note:      h.Note,
			UpdatedAt: h.UpdatedAt,
		})
	}

	trackingCacheMu.Lock()
	trackingCache[trackingID] = trackingCacheEntry{data: resp, at: time.Now()}
	// Buang entri basi sekalian — peta ini tidak pernah dibersihkan kalau
	// tidak, dan pesanan lama tidak akan pernah dibaca lagi.
	for k, v := range trackingCache {
		if time.Since(v.at) > 10*trackingTTL {
			delete(trackingCache, k)
		}
	}
	trackingCacheMu.Unlock()

	return resp, nil
}
