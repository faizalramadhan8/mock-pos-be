package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/email"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	// Midtrans deprecated 28 Jul 2026 — call site di-comment, package tetap
	// ada untuk backward-compat + rollback path kalau PG down.
	// "github.com/faizalramadhan/pos-be/internal/infrastructure/midtrans"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/pg"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomCheckoutService struct {
	DB      *gorm.DB
	Log     *zerolog.Logger
	PG      *pg.Client
	Email   *email.Client
	Voucher *EcomVoucherService
	AppURL  string
}

func NewEcomCheckoutService(ctx context.Context, db *gorm.DB) *EcomCheckoutService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomCheckoutService{
		DB:      db,
		Log:     logger,
		PG:      pg.NewClient(cfg.PGBaseURL, cfg.PGAppKey, cfg.PGAppSecret, cfg.PGMerchantName),
		Email:   email.NewClient(cfg.BrevoAPIKey, cfg.BrevoSenderEmail, cfg.BrevoSenderName),
		Voucher: NewEcomVoucherService(ctx, db),
		AppURL:  cfg.AppURL,
	}
}

// CreateOrder — end-to-end checkout: validate cart + address, create Order,
// decrement stock (reserve), clear cart, request Snap token.
// Kalau Midtrans belum configured, mode "manual" (customer transfer bank
// manual, Bu Santi verify).
// CartSubtotal — compute total dari cart user pakai harga effective ecom.
// Dipakai voucher validate endpoint supaya FE tidak perlu kirim subtotal
// (yang bisa di-tamper). Kalau selectedIDs non-empty, hanya sum item itu.
// `location` kosong = seluruh keranjang; diisi = hanya barang toko itu.
// Dipakai validasi voucher, yang minimum belanjanya harus dihitung dari
// subtotal satu toko saja.
func (s *EcomCheckoutService) CartSubtotal(userID string, selectedIDs []string, location string) (float64, *dto.ApiError) {
	var cart []entity.EcomCartItem
	q := s.DB.Preload("Product").Where("user_id = ?", userID)
	if len(selectedIDs) > 0 {
		q = q.Where("id IN ?", selectedIDs)
	}
	if err := q.Find(&cart).Error; err != nil {
		return 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
	}
	filterLoc := strings.TrimSpace(location)
	sub := 0.0
	for _, c := range cart {
		if c.Product == nil {
			continue
		}
		if filterLoc != "" && TempatSah(c.Product.Location) != TempatSah(filterLoc) {
			continue
		}
		// Lewat penghitung yang sama dengan CreateOrder. Kalau dihitung
		// terpisah, subtotal di keranjang bisa berbeda dari yang ditagih —
		// dan selisih seperti itu paling merusak kepercayaan saat ketahuan di
		// layar pembayaran.
		sub += computeEcomLinePrice(s.DB, c.Product, c.Quantity).LineTotal
	}
	return sub, nil
}

// CreateOrder — checkout lintas toko.
//
// Keranjang boleh campur: gula dari toko rumah + terigu dari Bekasi. Satu
// baris `orders` cuma muat SATU kurir, SATU ongkir, SATU resi, dan satu titik
// jemput — jadi tiap toko menghasilkan pesanannya sendiri, diikat
// `checkout_group_id` supaya satu pembayaran menutup semuanya. Itu juga yang
// Tokopedia/Shopee lakukan.
//
// Semuanya dalam SATU transaksi: kalau toko kedua gagal (stok keburu habis,
// voucher tidak valid), toko pertama ikut dibatalkan dan stoknya kembali.
// Separuh checkout yang berhasil jauh lebih buruk daripada checkout yang
// gagal utuh — customer sudah membayar sebagian tanpa tahu bagian mana.
func (s *EcomCheckoutService) CreateOrder(userID string, userEmail, userPhone, userName string, req dto.CheckoutCreateRequest) (*dto.CheckoutCreateResponse, *dto.ApiError) {
	// Load address (verify owner).
	var addr entity.EcomAddress
	if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.AddressID, userID).First(&addr).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Alamat tidak ditemukan"}
	}

	tx := s.DB.Begin()
	if tx.Error != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to start tx"}
	}

	// Build "checkout lines" — bisa dari 3 sumber (saling eksklusif):
	//   1. BuyNowItems (buy-now dari halaman produk) — lewati keranjang
	//   2. SelectedItemIDs (sebagian keranjang)
	//   3. Seluruh keranjang
	type checkoutLine struct {
		Product  *entity.Product
		Quantity int
		CartID   string // "" kalau buy-now
	}
	lines := []checkoutLine{}

	if len(req.BuyNowItems) > 0 {
		for _, bn := range req.BuyNowItems {
			var p entity.Product
			// Penyaring `location = 'home'` dihapus: buy-now sekarang bisa
			// datang dari toko mana pun. Yang menjaga tetap `ecom_is_available`
			// di bawah — itu yang menentukan barang boleh dijual online.
			if err := tx.Where("id = ? AND deleted_at IS NULL", bn.ProductID).First(&p).Error; err != nil {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Produk tidak ditemukan"}
			}
			lines = append(lines, checkoutLine{Product: &p, Quantity: bn.Quantity})
		}
	} else {
		var cart []entity.EcomCartItem
		q := tx.Preload("Product").Where("user_id = ?", userID)
		if len(req.SelectedItemIDs) > 0 {
			q = q.Where("id IN ?", req.SelectedItemIDs)
		}
		if err := q.Find(&cart).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
		}
		if len(cart) == 0 {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Tidak ada item yang dipilih"}
		}
		for _, c := range cart {
			lines = append(lines, checkoutLine{Product: c.Product, Quantity: c.Quantity, CartID: c.ID})
		}
	}

	// Kelompokkan per tempat.
	perTempat := map[string][]checkoutLine{}
	for _, ln := range lines {
		if ln.Product == nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Produk tidak valid"}
		}
		loc := TempatSah(ln.Product.Location)
		// Penjaga terakhir sebelum stok dipotong. Barang bisa saja masuk
		// keranjang sebelum tokonya dimatikan dari storefront; yang menentukan
		// adalah keadaan saat membayar, bukan saat memasukkan.
		if !TempatBolehTayang(s.DB, loc) {
			tx.Rollback()
			return nil, &dto.ApiError{
				StatusCode: fiber.ErrBadRequest,
				Message:    "Produk " + ln.Product.NameID + " sedang tidak dijual online",
			}
		}
		perTempat[loc] = append(perTempat[loc], ln)
	}

	// Pilihan pengiriman per tempat, dari request.
	pilihan := map[string]dto.CheckoutGroupRequest{}
	for _, g := range req.Groups {
		loc := TempatSah(g.Location)
		if _, dobel := pilihan[loc]; dobel {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Pengiriman untuk satu toko dikirim dua kali"}
		}
		pilihan[loc] = g
	}
	// Tiap toko di keranjang WAJIB punya pilihan pengiriman, dan sebaliknya
	// tidak boleh ada pilihan untuk toko yang tidak ada barangnya. Tanpa
	// pemeriksaan kedua, ongkir toko kosong tetap ditagihkan.
	for loc := range perTempat {
		if _, ada := pilihan[loc]; !ada {
			tx.Rollback()
			return nil, &dto.ApiError{
				StatusCode: fiber.ErrBadRequest,
				Message:    "Pengiriman untuk " + NamaToko(s.DB, loc) + " belum dipilih",
			}
		}
	}
	for loc := range pilihan {
		if len(perTempat[loc]) == 0 {
			tx.Rollback()
			return nil, &dto.ApiError{
				StatusCode: fiber.ErrBadRequest,
				Message:    "Tidak ada barang dari " + NamaToko(s.DB, loc) + " di keranjang",
			}
		}
	}

	checkoutGroupID := uuid.New().String()
	now := time.Now()
	expireAt := now.Add(24 * time.Hour) // 24 jam expire kalau tidak bayar
	ecomStatus := "pending_payment"

	biteshipAreaID := ""
	if addr.BiteshipAreaID != nil {
		biteshipAreaID = *addr.BiteshipAreaID
	}
	addrSnapshotBytes, _ := json.Marshal(map[string]interface{}{
		"label":            addr.Label,
		"recipient_name":   addr.RecipientName,
		"recipient_phone":  addr.RecipientPhone,
		"province":         addr.Province,
		"city":             addr.City,
		"district":         addr.District,
		"subdistrict":      addr.Subdistrict,
		"zipcode":          addr.Zipcode,
		"biteship_area_id": biteshipAreaID,
		"street_address":   addr.StreetAddress,
		"notes":            addr.Notes,
	})
	addrSnapshot := string(addrSnapshotBytes)

	consumedCartIDs := make([]string, 0, len(lines))
	ringkasan := make([]dto.CheckoutOrderSummary, 0, len(perTempat))
	orderIDs := make([]string, 0, len(perTempat))
	pgItems := make([]pg.CreatePaymentItem, 0, len(lines)+len(perTempat))
	grandTotal, totalSubtotal, totalOngkir, totalPotongan := 0.0, 0.0, 0.0, 0.0

	// Urutan tetap (rumah dulu, lalu toko) — iterasi map di Go acak, dan
	// pesanan yang nomor urutnya berubah-ubah tiap checkout menyulitkan
	// pencocokan dengan apa yang customer lihat di layar.
	for _, loc := range []string{TempatRumah, TempatToko} {
		grupLines := perTempat[loc]
		if len(grupLines) == 0 {
			continue
		}
		g := pilihan[loc]
		orderID := uuid.New().String()

		subtotal := 0.0
		orderItems := make([]entity.OrderItem, 0, len(grupLines))
		for _, ln := range grupLines {
			p := ln.Product
			if !p.EcomIsAvailable {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Produk %s tidak tersedia", p.NameID)}
			}
			if ln.Quantity > p.Stock {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Stok %s tidak cukup (tersisa %d)", p.NameID, p.Stock)}
			}
			// Harga grosir mode PAKET — lihat computeEcomLinePrice.
			lp := computeEcomLinePrice(tx, p, ln.Quantity)
			price := lp.UnitPrice
			regular := ecomBasePrice(p)
			subtotal += lp.LineTotal

			orderItems = append(orderItems, entity.OrderItem{
				ID:        uuid.New().String(),
				OrderID:   orderID,
				ProductID: p.ID,
				Name:      p.NameID,
				Quantity:  ln.Quantity,
				UnitType:  "individual",
				UnitPrice: price,
				// RegularPrice = harga sebelum grosir, supaya "hemat berapa"
				// bisa dihitung di laporan tanpa menebak harga saat itu.
				RegularPrice:  &regular,
				PurchasePrice: p.PurchasePrice,
				PriceSource:   lp.PriceSource,
				TierID:        lp.TierID,
				PaketCount:    lp.PaketCount,
				ExtraCount:    lp.ExtraCount,
			})

			// Guard di WHERE wajib: dua customer bisa checkout barang yang
			// sama bersamaan, dan `ln.Quantity > p.Stock` di atas sudah basi
			// begitu dibaca. `stock >= ?` membuat decrement gagal
			// (RowsAffected 0) alih-alih membuat stok minus.
			res := tx.Model(&entity.Product{}).Where("id = ? AND stock >= ?", p.ID, ln.Quantity).
				Update("stock", gorm.Expr("stock - ?", ln.Quantity))
			if res.Error != nil {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to reserve stock"}
			}
			if res.RowsAffected == 0 {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: fmt.Sprintf("Stok %s baru saja habis — silakan kurangi jumlahnya", p.NameID)}
			}
			// Pergerakan stok dicatat di TEMPAT produknya, bukan selalu
			// "home": sejak storefront melayani dua toko, mencatatnya di rak
			// yang salah membuat audit stok Bekasi bolong.
			if err := tx.Create(&entity.StockMovement{
				ID:        uuid.New().String(),
				ProductID: p.ID,
				Type:      "out",
				Quantity:  ln.Quantity,
				UnitType:  "individual",
				Location:  loc,
				UnitPrice: price,
				Reason:    "sale",
				Note:      fmt.Sprintf("Penjualan online (Order #%s)", orderID[:8]),
				CreatedBy: userID,
			}).Error; err != nil {
				tx.Rollback()
				return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to record stock movement"}
			}
			if ln.CartID != "" {
				consumedCartIDs = append(consumedCartIDs, ln.CartID)
			}
		}

		// Minimal belanja per toko. Tiap paket punya ongkos nyata sendiri,
		// jadi Rp 30rb dari dua toko bukan sama dengan Rp 60rb dari satu.
		// Diperiksa di sini, bukan cuma di FE: penjagaan yang hanya ada di
		// layar bisa dilewati siapa pun yang memanggil API langsung.
		if minOrder := s.minimalBelanja(loc); minOrder > 0 && subtotal < minOrder {
			tx.Rollback()
			return nil, &dto.ApiError{
				StatusCode: fiber.ErrBadRequest,
				Message: fmt.Sprintf("Minimal belanja di %s Rp %s", NamaToko(s.DB, loc),
					formatRupiahVoucher(minOrder)),
			}
		}

		// Voucher TOKO INI. Potongannya tidak pernah menyentuh toko lain.
		voucherDiscount := 0.0
		var voucherCodePtr *string
		if g.VoucherCode != "" {
			vd, v, verr := s.Voucher.ApplyOnCheckout(tx, g.VoucherCode, subtotal, loc)
			if verr != nil {
				tx.Rollback()
				return nil, verr
			}
			voucherDiscount = vd
			if v != nil {
				c := v.Code
				voucherCodePtr = &c
			}
		}

		total := subtotal - voucherDiscount + g.ShippingCost
		if total < 0 {
			total = 0
		}

		courierName := g.ShippingCourierName
		if courierName == "" {
			courierName = g.ShippingCourier
		}
		serviceName := g.ShippingServiceName
		if serviceName == "" {
			serviceName = g.ShippingService
		}
		courier, service, etd := g.ShippingCourier, g.ShippingService, g.ShippingETD
		grupID := checkoutGroupID
		ecomUserIDPtr := &userID
		ecomStatusPtr := &ecomStatus

		// order_source: 'home' untuk toko rumah, 'ecom' untuk Bekasi online.
		// Dua sumbu yang berbeda — lihat tempat.go.
		orderSource := "home"
		if loc == TempatToko {
			orderSource = "ecom"
		}

		order := entity.Order{
			ID:                      orderID,
			Items:                   orderItems,
			Subtotal:                subtotal,
			PPNRate:                 0,
			PPN:                     0,
			Total:                   total,
			Payment:                 "pending",
			Status:                  "pending",
			Customer:                userName,
			CustomerPhone:           userPhone,
			CreatedBy:               userID,
			OrderSource:             orderSource,
			CheckoutGroupID:         &grupID,
			EcomUserID:              ecomUserIDPtr,
			ShippingAddressSnapshot: &addrSnapshot,
			ShippingCourier:         &courier,
			ShippingCourierName:     &courierName,
			ShippingService:         &service,
			ShippingServiceName:     &serviceName,
			ShippingCost:            g.ShippingCost,
			ShippingETD:             &etd,
			EcomStatus:              ecomStatusPtr,
			PaymentExpiredAt:        &expireAt,
			VoucherCode:             voucherCodePtr,
			VoucherDiscount:         voucherDiscount,
		}
		if err := tx.Create(&order).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create order"}
		}

		for _, it := range orderItems {
			pgItems = append(pgItems, pg.CreatePaymentItem{
				ProductID: it.ProductID, ProductCode: it.ProductID, ProductName: it.Name,
				Price: it.UnitPrice, Quantity: it.Quantity,
			})
		}
		if g.ShippingCost > 0 {
			pgItems = append(pgItems, pg.CreatePaymentItem{
				ProductID: "shipping", ProductCode: "shipping",
				ProductName: fmt.Sprintf("Ongkir %s %s", courierName, serviceName),
				Price:       g.ShippingCost, Quantity: 1,
			})
		}

		voucherCodeStr := ""
		if voucherCodePtr != nil {
			voucherCodeStr = *voucherCodePtr
		}
		ringkasan = append(ringkasan, dto.CheckoutOrderSummary{
			OrderID:         orderID,
			Location:        loc,
			StoreName:       NamaToko(s.DB, loc),
			Subtotal:        subtotal,
			ShippingCost:    g.ShippingCost,
			VoucherCode:     voucherCodeStr,
			VoucherDiscount: voucherDiscount,
			Total:           total,
		})
		orderIDs = append(orderIDs, orderID)
		grandTotal += total
		totalSubtotal += subtotal
		totalOngkir += g.ShippingCost
		totalPotongan += voucherDiscount
	}

	if len(orderIDs) == 0 {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Tidak ada item yang dipilih"}
	}

	// ── Pembayaran: SEKALI untuk seluruh grup ────────────────────────────
	//
	// Saklar PG dibaca dari baris RUMAH, yang berperan sebagai saklar
	// pembayaran storefront. Alasannya rekening penampungnya satu dan milik
	// rumah (keputusan 30 Okt 2026) — jadi mode pembayaran memang keputusan
	// satu tingkat di atas toko, bukan per toko.
	//
	// Best-effort: kalau baris pengaturannya tidak terbaca, anggap PG aktif
	// (perilaku lama) supaya checkout tidak mati gara-gara tabel pengaturan.
	pgEnabled := true
	var ecomSt entity.EcomSettings
	if err := s.DB.Where("location = ?", TempatRumah).First(&ecomSt).Error; err == nil {
		pgEnabled = ecomSt.PaymentPgEnabled
	}

	paymentMode := "pg"
	paymentURL := ""
	paymentRef := ""
	channelSaved := req.PaymentChannel
	categorySaved := req.PaymentChannelCategory

	// Kanal wajib HANYA saat PG menyala. Di sini, bukan di validasi struct,
	// karena cuma di titik ini saklarnya diketahui.
	if pgEnabled && req.PaymentChannel == "" {
		tx.Rollback()
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Metode pembayaran belum dipilih"}
	}

	if !pgEnabled {
		// Transfer manual: jangan panggil PG sama sekali. Memanggilnya tetap
		// gagal dan jatuh ke manual juga, tapi membuang 20 detik timeout di
		// depan customer dan mengotori log dengan error yang bukan error.
		paymentMode = "manual"
		channelSaved = ""
		categorySaved = ""
		s.Log.Info().Str("checkout_group", checkoutGroupID).Msg("PG dimatikan di pengaturan — checkout mode transfer manual")
	} else {
		callbackURL := strings.TrimRight(s.AppURL, "/") + "/api/v1/ecom/payments/webhook/pg"
		successURL := strings.TrimRight(s.AppURL, "/") + "/shop/pesanan/" + orderIDs[0] + "?paid=1"
		// Merchant name — samaran per request Bu Santi 28 Jul 2026.
		merchantName := s.PG.MerchantName
		if merchantName == "" {
			merchantName = "Testing"
		}
		pgReq := pg.CreatePaymentRequest{
			Channel:     req.PaymentChannel,
			TotalAmount: grandTotal,
			// TrxID = id GRUP, bukan id pesanan: satu pembayaran menutup
			// semua pesanan di dalamnya. Webhook mencarinya lewat
			// checkout_group_id, dengan jalur lama (id pesanan) tetap
			// dikenali untuk pesanan sebelum perubahan ini.
			TrxID:          checkoutGroupID,
			BillingEmail:   userEmail,
			BillingName:    addr.RecipientName,
			BillingPhone:   addr.RecipientPhone,
			BillingAddress: addr.StreetAddress,
			BillingID:      userID,
			BillingUID:     userID,
			CallbackURL:    callbackURL,
			SuccessURL:     successURL,
			Description:    fmt.Sprintf("Order #%s", orderIDs[0][:8]),
			Merchant:       merchantName,
			Remark:         "",
			Item:           pgItems,
		}
		pgCtx, pgCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer pgCancel()
		pgResp, pgErr := s.PG.CreatePayment(pgCtx, pgReq)
		if pgErr != nil {
			s.Log.Warn().Err(pgErr).Str("checkout_group", checkoutGroupID).Str("channel", req.PaymentChannel).Msg("PG create payment failed, fallback manual")
		}
		if s.PG.IsConfigured() && pgResp != nil && pgResp.PaymentURL != "" {
			paymentURL = pgResp.PaymentURL
		} else {
			paymentMode = "manual"
		}
		if pgResp != nil {
			paymentRef = pgResp.PaymentID
		}
	}

	// Snapshot PG ke SEMUA pesanan dalam grup — tiap pesanan harus bisa
	// menampilkan tautan bayar yang sama.
	pgUpdate := map[string]interface{}{
		"payment_channel":          channelSaved,
		"payment_channel_category": categorySaved,
	}
	if paymentURL != "" {
		pgUpdate["payment_url"] = paymentURL
	}
	if paymentRef != "" {
		pgUpdate["payment_reference"] = paymentRef
	}
	if err := tx.Model(&entity.Order{}).Where("checkout_group_id = ?", checkoutGroupID).Updates(pgUpdate).Error; err != nil {
		s.Log.Warn().Err(err).Msg("Failed to save PG snapshot")
	}

	// Hapus HANYA item keranjang yang ikut di-checkout — sisanya tetap
	// tersimpan untuk checkout berikutnya. Buy-now tidak menyentuh keranjang.
	if len(consumedCartIDs) > 0 {
		if err := tx.Where("user_id = ? AND id IN ?", userID, consumedCartIDs).Delete(&entity.EcomCartItem{}).Error; err != nil {
			tx.Rollback()
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to clear cart"}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to commit"}
	}

	return &dto.CheckoutCreateResponse{
		CheckoutGroupID: checkoutGroupID,
		Orders:          ringkasan,
		OrderID:         orderIDs[0],
		GrandTotal:      grandTotal,
		Total:           grandTotal,
		Subtotal:        totalSubtotal,
		ShippingCost:    totalOngkir,
		VoucherDiscount: totalPotongan,
		PaymentURL:      paymentURL,
		PaymentChannel:  channelSaved,
		PaymentMode:     paymentMode,
		EcomStatus:      ecomStatus,
	}, nil
}

// minimalBelanja — ambang belanja satu toko. 0 = tidak ada batas.
func (s *EcomCheckoutService) minimalBelanja(location string) float64 {
	var st entity.EcomSettings
	if err := s.DB.Select("min_order_amount").Where("location = ?", TempatSah(location)).First(&st).Error; err != nil {
		return 0
	}
	return st.MinOrderAmount
}

// HandlePGNotification — webhook DOKU (via alifworks PG wrapper) setiap
// payment status change. Update ecom_status + paid_at. Rollback stock kalau
// FAILED/VOIDED.
//
// DOKU response format kita return: {responseCode, responseMessage} — handler
// yang bungkus (bukan default ApiResponse envelope) supaya DOKU parse OK.
func (s *EcomCheckoutService) HandlePGNotification(notif pg.WebhookPayload) *dto.ApiError {
	ref := notif.Order.InvoiceNumber
	if ref == "" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Missing invoice_number"}
	}

	// invoice_number bisa berarti dua hal:
	//   - id GRUP checkout (sejak 30 Okt 2026) — satu pembayaran, bisa lebih
	//     dari satu pesanan
	//   - id pesanan (pesanan lama, sebelum checkout lintas toko)
	// Dicoba grup dulu; kalau kosong, jatuh ke id pesanan.
	var orders []entity.Order
	if err := s.DB.Where("checkout_group_id = ? AND deleted_at IS NULL", ref).Find(&orders).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to load orders"}
	}
	if len(orders) == 0 {
		var one entity.Order
		if err := s.DB.Where("id = ? AND deleted_at IS NULL", ref).First(&one).Error; err != nil {
			return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order not found"}
		}
		orders = []entity.Order{one}
	}

	// Dulu di sini `order.OrderSource != "ecom"` — yang menolak SEMUA pesanan
	// storefront sejak 21 Okt 2026, karena sejak itu pesanan toko rumah
	// bersumber 'home'. Tidak pernah ketahuan hanya karena PG sedang
	// dimatikan; begitu dinyalakan lagi, setiap pembayaran akan ditolak dan
	// pesanan yang sudah dibayar tetap menggantung "belum bayar".
	for i := range orders {
		if orders[i].OrderSource != "ecom" && orders[i].OrderSource != "home" {
			return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Not an ecom order"}
		}
	}

	newStatus := ""
	shouldRestoreStock := false
	shouldMarkPaid := false

	// DOKU transaction status: SUCCESS / FAILED / VOIDED. Pending state tidak
	// ada karena PG hanya kirim event saat customer submit action (bukan saat
	// initial VA generate).
	switch strings.ToUpper(notif.Transaction.Status) {
	case "SUCCESS":
		newStatus = "paid"
		shouldMarkPaid = true
	case "FAILED":
		newStatus = "cancelled"
		shouldRestoreStock = true
	case "VOIDED":
		newStatus = "cancelled"
		shouldRestoreStock = true
	}
	if newStatus == "" {
		return nil // ignore unknown status
	}

	tx := s.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	diproses := make([]string, 0, len(orders))
	for i := range orders {
		order := orders[i]
		orderID := order.ID

		// Idempotency PER PESANAN — webhook bisa datang dua kali, dan dalam
		// satu grup bisa saja satu pesanan sudah ter-update duluan.
		currentEcom := ""
		if order.EcomStatus != nil {
			currentEcom = *order.EcomStatus
		}
		if currentEcom == newStatus {
			s.Log.Info().Str("order_id", orderID).Str("status", newStatus).Msg("PG webhook: duplicate, skip")
			continue
		}

		updates := map[string]interface{}{"ecom_status": newStatus}
		if notif.Transaction.OriginalRequestID != "" {
			updates["payment_reference"] = notif.Transaction.OriginalRequestID
		}
		if shouldMarkPaid {
			now := time.Now()
			updates["payment_paid_at"] = now
			updates["status"] = "completed" // POS side status
		}
		if err := tx.Model(&entity.Order{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
			tx.Rollback()
			return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update order"}
		}

		if shouldRestoreStock {
			var items []entity.OrderItem
			tx.Where("order_id = ?", orderID).Find(&items)
			// Tempat pergerakan diambil dari pesanannya, bukan selalu "home".
			// Mencatat pengembalian stok Bekasi di rak rumah membuat dua
			// audit stok salah sekaligus.
			loc := TempatDariOrderSource(order.OrderSource)
			for _, it := range items {
				if it.ProductID == "" {
					continue
				}
				tx.Model(&entity.Product{}).Where("id = ?", it.ProductID).
					Update("stock", gorm.Expr("stock + ?", it.Quantity))
				tx.Create(&entity.StockMovement{
					ID:        uuid.New().String(),
					ProductID: it.ProductID,
					Type:      "in",
					Quantity:  it.Quantity,
					UnitType:  "individual",
					Location:  loc,
					UnitPrice: it.UnitPrice,
					Reason:    "cancel",
					Note:      fmt.Sprintf("Pembayaran online gagal/kedaluwarsa (Order #%s)", orderID[:8]),
					CreatedBy: order.CreatedBy,
				})
			}
		}
		diproses = append(diproses, orderID)
	}

	if len(diproses) == 0 {
		tx.Rollback()
		return nil // semuanya sudah di status itu — tidak ada yang perlu diubah
	}

	if err := tx.Commit().Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to commit"}
	}

	s.Log.Info().
		Strs("order_ids", diproses).
		Str("new_status", newStatus).
		Str("pg_status", notif.Transaction.Status).
		Str("channel", notif.Channel.ID).
		Msg("PG notification processed")

	// Email konfirmasi per pesanan — best-effort di goroutine, tidak block
	// response webhook (DOKU retry kalau webhook lambat > 5 detik). Satu
	// email per pesanan karena tiap pesanan punya kurir dan resinya sendiri.
	if shouldMarkPaid {
		for _, id := range diproses {
			go s.sendOrderPaidEmail(id)
		}
		// Sprint 2 #10 (loyalty ecom) di-HOLD atas permintaan Bu Santi
		// 30 Jul 2026 — flow member phone matching perlu review dulu.
	}

	return nil
}

// sendOrderPaidEmail — dipanggil setelah order transition ke paid.
// Fetch order + user + items, render HTML sederhana, kirim via Brevo.
// Kalau Brevo tidak configured → Email.Send stub log ke stdout, tetap OK.
func (s *EcomCheckoutService) sendOrderPaidEmail(orderID string) {
	var order entity.Order
	if err := s.DB.Preload("Items").Where("id = ?", orderID).First(&order).Error; err != nil {
		s.Log.Warn().Err(err).Str("order_id", orderID).Msg("email: order fetch failed")
		return
	}
	if order.EcomUserID == nil || *order.EcomUserID == "" {
		return
	}
	var user entity.User
	if err := s.DB.Where("id = ?", *order.EcomUserID).First(&user).Error; err != nil {
		return
	}
	if user.Email == "" {
		return
	}

	// Recipient info dari shipping snapshot.
	recipientName := user.FullName
	if order.ShippingAddressSnapshot != nil {
		var snap map[string]interface{}
		if err := json.Unmarshal([]byte(*order.ShippingAddressSnapshot), &snap); err == nil {
			if n, ok := snap["recipient_name"].(string); ok && n != "" {
				recipientName = n
			}
		}
	}

	shortID := orderID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	// Build items row HTML.
	itemsHTML := ""
	for _, it := range order.Items {
		itemsHTML += fmt.Sprintf(
			`<tr><td style="padding:6px 0">%s <span style="color:#999">×%d</span></td><td align="right" style="padding:6px 0">Rp %s</td></tr>`,
			it.Name, it.Quantity, formatRpForEmail(it.UnitPrice*float64(it.Quantity)))
	}
	trackingURL := s.AppURL
	if trackingURL == "" {
		trackingURL = "https://tbksanti.id"
	}
	trackingURL += "/shop/pesanan/" + orderID

	subject := "Pembayaran diterima — Order #" + shortID + " TBK Santi"
	html := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:520px;margin:0 auto;padding:24px">
<h2 style="color:#C4302B;margin:0 0 8px">Pembayaran Diterima</h2>
<p>Hai %s, terima kasih!</p>
<p>Pembayaran untuk order <b>#%s</b> sudah kami terima. Pesananmu segera diproses.</p>
<table style="width:100%%;border-collapse:collapse;font-size:14px;margin:12px 0">
%s
<tr><td colspan="2" style="border-top:1px solid #eee;padding-top:8px"></td></tr>
<tr><td>Subtotal</td><td align="right">Rp %s</td></tr>
<tr><td>Ongkir</td><td align="right">Rp %s</td></tr>
<tr style="font-weight:900;color:#C4302B"><td>Total</td><td align="right">Rp %s</td></tr>
</table>
<a href="%s" style="display:inline-block;padding:10px 20px;background:#C4302B;color:#fff;border-radius:8px;text-decoration:none;font-weight:bold;margin-top:8px">Lihat Detail Pesanan</a>
<p style="color:#999;font-size:12px;margin-top:24px">Toko Bahan Kue Santi · tbksanti.id</p>
</div>`,
		recipientName, shortID, itemsHTML,
		formatRpForEmail(order.Subtotal), formatRpForEmail(order.ShippingCost), formatRpForEmail(order.Total),
		trackingURL)

	text := fmt.Sprintf("Order #%s: pembayaran diterima. Total Rp %s. Lihat detail: %s",
		shortID, formatRpForEmail(order.Total), trackingURL)

	if err := s.Email.Send(user.Email, recipientName, subject, html, text); err != nil {
		s.Log.Warn().Err(err).Str("order_id", orderID).Str("email", user.Email).Msg("order paid email send failed")
	}
}

// formatRpForEmail — simple integer format dengan dot separator.
// Tidak pakai locale-aware library supaya dependency ringan.
func formatRpForEmail(n float64) string {
	i := int64(n)
	s := fmt.Sprintf("%d", i)
	// Insert dot tiap 3 digit dari kanan.
	if len(s) <= 3 {
		return s
	}
	out := ""
	for j, c := range []rune(s) {
		if j > 0 && (len(s)-j)%3 == 0 {
			out += "."
		}
		out += string(c)
	}
	return out
}
