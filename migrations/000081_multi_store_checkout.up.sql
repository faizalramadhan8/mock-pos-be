-- Checkout lintas toko: satu keranjang, banyak toko, satu pembayaran.
--
-- Satu baris `orders` cuma muat SATU kurir, SATU ongkir, SATU resi, dan satu
-- titik jemput. Jadi keranjang berisi dua toko menghasilkan DUA pesanan —
-- sama seperti Tokopedia/Shopee. Yang mengikatnya jadi satu transaksi adalah
-- kolom di bawah.

-- checkout_group_id: pesanan yang lahir dari satu kali klik "Buat Pesanan".
-- NULL untuk semua pesanan lama dan untuk pesanan POS — tidak ada yang perlu
-- di-backfill.
--
-- Gunanya: customer transfer SEKALI untuk dua pesanan, jadi satu bukti
-- transfer harus berlaku untuk keduanya, dan nominal yang dicocokkan Bu Santi
-- adalah jumlah keduanya. Tanpa pengikat ini dia akan melihat dua tagihan
-- terpisah untuk satu transfer dan berisiko mengira uangnya kurang.
ALTER TABLE orders
  ADD COLUMN checkout_group_id VARCHAR(36) NULL AFTER order_source;

CREATE INDEX idx_orders_checkout_group ON orders(checkout_group_id);

-- Voucher milik satu toko. Bu Santi 30 Okt 2026: "voucher itu dapat dari tiap
-- toko yang ingin membuat kode voucher, jadi harganya otomatis terpotong dari
-- toko itu jangan sampai yang kepotong malah toko lain."
--
-- NOT NULL dan tanpa nilai "berlaku di semua toko": voucher global di
-- keranjang dua toko memaksa potongannya dibagi, dan itu persis yang diminta
-- untuk tidak terjadi.
--
-- Baris lama diisi 'home' — storefront hari ini melayani toko rumah, jadi
-- voucher yang sudah terlanjur dibuat memang ditujukan ke sana. Tidak ada
-- perilaku yang berubah diam-diam.
ALTER TABLE ecom_vouchers
  ADD COLUMN location VARCHAR(10) NOT NULL DEFAULT 'home' AFTER code;

CREATE INDEX idx_ecom_vouchers_location ON ecom_vouchers(location);
