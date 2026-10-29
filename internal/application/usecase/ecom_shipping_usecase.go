package usecase

import (
	"context"
	"strings"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/biteship"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomShippingService struct {
	DB       *gorm.DB
	Log      *zerolog.Logger
	Biteship *biteship.Client
}

func NewEcomShippingService(ctx context.Context, db *gorm.DB) *EcomShippingService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomShippingService{
		DB:       db,
		Log:      logger,
		Biteship: NewBiteshipClient(cfg),
	}
}

// NewBiteshipClient — shared factory supaya multiple service pakai config
// yang sama (shipping rate + admin orders create + webhook verify).
func NewBiteshipClient(cfg *config.Config) *biteship.Client {
	return biteship.NewClient(
		cfg.BiteshipAPIKey,
		cfg.BiteshipOriginArea,
		cfg.BiteshipOriginPostal,
		cfg.BiteshipCouriers,
		cfg.ShippingFlatBaseRate,
	).WithShipper(
		cfg.BiteshipOriginName,
		cfg.BiteshipOriginPhone,
		cfg.BiteshipOriginAddress,
		cfg.BiteshipOriginNote,
	).WithWebhookAuth(
		cfg.BiteshipWebhookHeader,
		cfg.BiteshipWebhookSecret,
	)
}

// biteshipWithStoreOrigin — klien Biteship dengan titik asal diambil dari
// ecom_settings (yang bisa Bu Santi ubah sendiri di panel admin), jatuh ke
// nilai env untuk field yang belum diisi.
//
// Dibaca per pemakaian, bukan sekali saat service dibuat: kalau di-cache,
// mengubah alamat penjemputan tetap menuntut restart backend — persis
// keterbatasan yang bikin isian di layar itu percuma selama ini.
//
// Best-effort: kalau query gagal atau barisnya belum ada, pakai env apa
// adanya. Pengiriman tidak boleh mati gara-gara tabel pengaturan.
//
// `location` menentukan toko MANA yang jadi titik jemput. Sejak storefront
// melayani dua toko, memanggilnya tanpa tempat yang benar berarti kurir
// dikirim ke alamat yang salah dan ongkirnya dihitung dari jarak yang salah —
// dua-duanya tanpa pesan salah apa pun.
func biteshipWithStoreOrigin(db *gorm.DB, base *biteship.Client, location string) *biteship.Client {
	if db == nil || base == nil {
		return base
	}
	var st entity.EcomSettings
	if err := db.Where("location = ?", TempatSah(location)).First(&st).Error; err != nil {
		return base
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	// Nama pengirim ikut nama toko — yang tertera di resi dan dilihat kurir.
	// Koordinat dipasang terpisah: tanpa ini kurir instan tidak muncul.
	return base.WithOriginCoords(st.StorePickupLatitude, st.StorePickupLongitude).WithOriginOverride(
		deref(st.StorePickupAreaID),
		"", // ecom_settings belum punya kode pos; area_id sudah cukup untuk Biteship
		strings.TrimSpace(st.StoreName),
		deref(st.StorePickupPhone),
		deref(st.StorePickupAddress),
		"",
	)
}

// Shipping — klien siap pakai, titik jemputnya milik `location`.
func (s *EcomShippingService) Shipping(location string) *biteship.Client {
	return biteshipWithStoreOrigin(s.DB, s.Biteship, location)
}

// SearchAreas — proxy ke Biteship Maps API. Dipakai FE address form untuk
// autocomplete "Kelurahan". FE debounce ~300ms sebelum call supaya tidak
// spam Biteship (docs bilang trigger setelah user selesai ngetik).
// Return array minimal 0, non-nil.
func (s *EcomShippingService) SearchAreas(input string) ([]biteship.Area, *dto.ApiError) {
	if len(input) < 3 {
		return []biteship.Area{}, nil
	}
	// Pencarian wilayah tidak ada hubungannya dengan titik jemput — cuma
	// proxy ke Maps API. Tempat mana pun menghasilkan klien yang sama.
	cl := s.Shipping(TempatRumah)
	if !cl.IsConfigured() {
		return []biteship.Area{}, nil
	}
	areas, err := cl.SearchAreas(input)
	if err != nil {
		s.Log.Warn().Err(err).Str("input", input).Msg("Biteship SearchAreas failed")
		// Kembalikan empty saja — cegah address form stuck loading. FE fallback:
		// customer input postal manual, kita cover via postal_code path.
		return []biteship.Area{}, nil
	}
	if areas == nil {
		areas = []biteship.Area{}
	}
	return areas, nil
}

// GetRates — ongkir per TOKO, bukan satu angka untuk seluruh keranjang.
//
// Keranjang boleh campur: gula dari toko rumah + terigu dari Bekasi. Tiap
// toko punya alamat jemput sendiri, jadi jaraknya beda, tarifnya beda, dan
// customer memilih kurir untuk masing-masing. Hasilnya dua pesanan saat
// checkout — satu `orders` cuma muat satu kurir, satu ongkir, satu resi.
func (s *EcomShippingService) GetRates(userID string, req dto.ShippingRateRequest) (*dto.ShippingRatesResponse, *dto.ApiError) {
	var addr entity.EcomAddress
	if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.AddressID, userID).First(&addr).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Alamat tidak ditemukan"}
	}

	// Barangnya bisa datang dari tiga sumber — sama seperti CreateOrder:
	//   1. BuyNowItems (langsung dari halaman produk, lewati keranjang)
	//   2. SelectedItemIDs (sebagian keranjang)
	//   3. Seluruh keranjang
	type kelompok struct {
		items    []biteship.Item
		weight   int
		subtotal float64
	}
	perTempat := map[string]*kelompok{}

	tambah := func(p *entity.Product, qty int) {
		if p == nil || qty <= 0 {
			return
		}
		w := 200
		if p.EcomWeightGrams != nil && *p.EcomWeightGrams > 0 {
			w = *p.EcomWeightGrams
		}
		// Harga baris memakai penghitung yang sama dengan keranjang dan
		// checkout — kalau beda, nilai barang yang dilaporkan ke kurir
		// (dipakai asuransi) tidak cocok dengan yang ditagih.
		hrg := computeEcomLinePrice(s.DB, p, qty)
		loc := TempatSah(p.Location)
		g := perTempat[loc]
		if g == nil {
			g = &kelompok{}
			perTempat[loc] = g
		}
		g.items = append(g.items, biteship.Item{
			Name: p.Name, Value: int(hrg.UnitPrice), Weight: w, Quantity: qty,
		})
		g.weight += w * qty
		g.subtotal += hrg.LineTotal
	}

	if len(req.BuyNowItems) > 0 {
		for _, bn := range req.BuyNowItems {
			var p entity.Product
			if err := s.DB.Where("id = ? AND deleted_at IS NULL", bn.ProductID).First(&p).Error; err == nil {
				tambah(&p, bn.Quantity)
			}
		}
	} else {
		var cart []entity.EcomCartItem
		q := s.DB.Preload("Product").Where("user_id = ?", userID)
		if len(req.SelectedItemIDs) > 0 {
			q = q.Where("id IN ?", req.SelectedItemIDs)
		}
		if err := q.Find(&cart).Error; err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
		}
		for _, c := range cart {
			tambah(c.Product, c.Quantity)
		}
	}

	resp := &dto.ShippingRatesResponse{Groups: []dto.ShippingRateGroup{}}
	resp.Address.Label = addr.Label
	resp.Address.RecipientName = addr.RecipientName
	resp.Address.City = addr.City
	resp.Address.Province = addr.Province

	// Urutan tetap (rumah dulu, lalu toko) supaya tampilan checkout tidak
	// berubah-ubah urutannya tiap kali halaman dimuat — iterasi map di Go
	// urutannya acak.
	for _, loc := range []string{TempatRumah, TempatToko} {
		g := perTempat[loc]
		if g == nil || len(g.items) == 0 {
			continue
		}
		berat := g.weight
		if berat == 0 {
			berat = 1000
		}
		grup := dto.ShippingRateGroup{
			Location:         loc,
			StoreName:        NamaToko(s.DB, loc),
			TotalWeightGrams: berat,
			Subtotal:         g.subtotal,
			Rates:            []dto.ShippingRate{},
		}
		resp.TotalWeightGrams += berat

		// Toko yang sedang tidak tayang: jangan panggil Biteship sama sekali,
		// tapi tetap tampilkan kelompoknya dengan alasannya. Menghilangkannya
		// diam-diam membuat customer melihat keranjang yang isinya tidak
		// pernah bisa dibayar tanpa tahu sebabnya.
		if !TempatBolehTayang(s.DB, loc) {
			grup.Error = grup.StoreName + " sedang tidak melayani pesanan online. Hapus barangnya dari keranjang untuk melanjutkan."
			resp.Groups = append(resp.Groups, grup)
			continue
		}

		rateReq := biteship.RateRequest{
			DestinationPostal: addr.Zipcode,
			DestinationLat:    addr.Latitude,
			DestinationLng:    addr.Longitude,
			Items:             g.items,
			TotalWeightGrams:  berat,
		}
		if addr.BiteshipAreaID != nil && *addr.BiteshipAreaID != "" {
			rateReq.DestinationAreaID = *addr.BiteshipAreaID
		}
		rates, err := s.Shipping(loc).GetRates(rateReq)
		if err != nil {
			// Kegagalan satu toko tidak mematikan toko lain. Customer masih
			// bisa checkout yang satunya; kalau seluruh halaman dimatikan,
			// dia tidak bisa belanja sama sekali.
			s.Log.Error().Err(err).Str("location", loc).Msg("Failed to get shipping rates")
			grup.Error = "Ongkir dari " + grup.StoreName + " belum bisa dihitung. Coba lagi sebentar lagi."
			resp.Groups = append(resp.Groups, grup)
			continue
		}
		for _, r := range rates {
			grup.Rates = append(grup.Rates, dto.ShippingRate{
				Courier: r.Courier, CourierName: r.CourierName,
				Service: r.Service, ServiceName: r.ServiceName,
				Cost: r.Cost, ETD: r.ETD,
			})
		}
		if len(grup.Rates) == 0 && grup.Error == "" {
			grup.Error = "Tidak ada kurir yang melayani rute ini dari " + grup.StoreName + "."
		}
		resp.Groups = append(resp.Groups, grup)
	}

	return resp, nil
}
