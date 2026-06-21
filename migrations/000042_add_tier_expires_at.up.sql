-- Per request Bu Santi 21 Jun 2026 — tier harga khusus bisa di-set timing:
-- 1/3/6/12/30 hari, lewat itu auto-balik ke harga normal. Use case: promo
-- terbatas ("beli 3 = 33rb sampai akhir minggu").
--
-- Tier dengan expires_at > NOW() = aktif, < NOW() = expired (skip di POS
-- compute, tapi tetap visible di admin catalog untuk audit + extend).
-- NULL = tidak terbatas (default existing tiers + opsi "tidak terbatas").
ALTER TABLE product_price_tiers
  ADD COLUMN expires_at DATETIME NULL,
  ADD INDEX idx_expires_at (expires_at);

-- Tier history juga ikut snapshot expires_at supaya audit lengkap.
ALTER TABLE product_price_tier_history
  ADD COLUMN expires_at DATETIME NULL;
