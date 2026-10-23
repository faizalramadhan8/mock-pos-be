-- Toko rumah jadi unit usaha sendiri, dengan KATALOG PRODUK TERPISAH.
-- (Bu Santi 13 + 18 Okt 2026, bentuk akhir disepakati Fai 21 Okt 2026.)
--
-- "Untuk dirumah saya hanya stok beberapa produk aja. Dan stoknya terpisah
--  dengan toko bekasi" — 13 Okt
-- "yang online rumah tidak disatukan dengan transaksi toko bekasi. Karna
--  kalau disatukan agak ribet nanti pas closing toko bekasi" — 18 Okt
--
-- Keputusan bentuknya: tiap tempat punya katalog produknya SENDIRI. POS
-- membuat dan melihat produk Bekasi; panel rumah membuat dan melihat produk
-- rumah; keduanya tidak saling tahu. Barang yang kebetulan dijual di dua
-- tempat = dua baris produk, satu milik masing-masing.
--
-- Rancangan sebelumnya memakai dua kolom stok dalam satu baris produk
-- (`stock` + `stock_home`). Itu lahir dari asumsi "satu barang, dua rak" —
-- dan asumsi itu gugur begitu katalognya dipisah. Dua kolom juga membawa
-- tiga cacat yang semuanya hilang di bentuk ini:
--   1. Harga grosir bocor — `product_price_tiers` dibaca POS dan storefront
--      dari baris yang sama, jadi tier kartonan untuk rumah ikut berlaku di
--      konter Bekasi. Melawan alasan Bu Santi membuka toko rumah.
--   2. `purchase_price` satu angka untuk dua tempat → laba per tempat jadi
--      perkiraan.
--   3. `ecom_price` satu kolom, padahal storefront Bekasi (habis Lebaran)
--      dan storefront rumah butuh harga yang berbeda.
-- Dengan katalog terpisah, ketiganya selesai tanpa kolom tambahan apa pun.

-- ── Tempat pemilik produk ────────────────────────────────────────────────
-- 'store' = Bekasi (kasir + nanti storefront Bekasi), 'home' = toko rumah.
-- Seluruh produk yang sudah ada milik Bekasi — benar, toko rumah belum
-- pernah punya katalog.
--
-- POS menyaring `location='store'` di daftar produk, pencarian, scan
-- barcode, stok, dan peringatan Beranda. Storefront rumah menyaring
-- `location='home'`. Penyaring di scan barcode yang paling menentukan:
-- pencariannya mengambil yang pertama cocok, jadi tanpa itu kasir bisa
-- menjual barang yang fisiknya ada di rumah.
ALTER TABLE products
  ADD COLUMN location VARCHAR(10) NOT NULL DEFAULT 'store' AFTER is_active;

CREATE INDEX idx_products_location ON products(location, is_active);
-- Storefront: produk rumah yang tayang dan masih ada stoknya.
CREATE INDEX idx_products_loc_avail_stock ON products(location, ecom_is_available, stock);

-- ── Tempat pada tiap pergerakan stok ─────────────────────────────────────
-- Pergerakan stok rumah HANYA terlihat di panel admin rumah (keputusan Fai
-- 21 Okt 2026); POS menyaringnya keluar. Kolom ini yang memisahkannya.
-- Semua baris lama = 'store'.
ALTER TABLE stock_movements
  ADD COLUMN location VARCHAR(10) NOT NULL DEFAULT 'store' AFTER unit_type;

CREATE INDEX idx_stock_movements_location ON stock_movements(location, created_at);

-- ── Rekening per tempat ──────────────────────────────────────────────────
-- Toko rumah memakai rekening terpisah. Tanpa penanda ini storefront
-- menampilkan SEMUA rekening termasuk punya Bekasi, dan sebagian customer
-- pasti transfer ke yang salah — pembukuan yang sengaja dipisah tercampur
-- lagi di langkah paling awal.
ALTER TABLE bank_accounts
  ADD COLUMN location VARCHAR(10) NOT NULL DEFAULT 'store' AFTER settings_id;
