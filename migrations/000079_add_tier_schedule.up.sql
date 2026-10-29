-- Jadwal berlaku tier harga grosir.
--
-- Bu Santi 30 Okt 2026: "Ada periode tanggal dan bulan. Jadi saya bisa buat
-- promo dalam bulan mendatang." Sampai sekarang tier hanya punya `expires_at`
-- yang dihitung dari NOW + durasi (1/3/6/12/30 hari), jadi promo SELALU mulai
-- detik itu juga. Tidak ada cara memasang promo Desember di bulan November
-- selain menunggu sampai 1 Desember lalu memasangnya manual.
--
-- `starts_at` NULL = berlaku sejak dibuat (semua tier lama). Tier dengan
-- starts_at di masa depan dilewati oleh SEMUA penghitung harga dan tidak
-- dikirim ke storefront.
ALTER TABLE product_price_tiers
  ADD COLUMN starts_at DATETIME NULL AFTER target_type;

CREATE INDEX idx_product_price_tiers_starts ON product_price_tiers(starts_at);

-- Tabel audit ikut menyimpannya supaya snapshot tiap versi tier lengkap.
-- Catatan: `start_date` di tabel ini adalah masa berlaku BARIS AUDIT-nya
-- (kapan versi ini tercatat), sementara `starts_at` adalah jadwal TIER-nya.
-- Dua hal berbeda yang kebetulan bertetangga.
ALTER TABLE product_price_tier_history
  ADD COLUMN starts_at DATETIME NULL AFTER target_type;
