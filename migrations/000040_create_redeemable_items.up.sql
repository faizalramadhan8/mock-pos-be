-- Redeemable items — barang khusus tebus poin yang admin set manual.
-- TERPISAH dari katalog produk POS (`products` table) per request Bu Santi
-- 21 Jun 2026 — barang tebus bukan barang jual normal, biasanya merchandise
-- (mug, kaos, voucher, hampers) yang admin siapin khusus untuk reward.
--
-- points_cost: admin set bebas (tidak terkait sellingPrice).
-- stock: qty available untuk redeem; auto-decrement saat customer tebus.
-- redeemed: counter audit total qty yang sudah ke-redeem (cumulative).
-- deleted_at: soft delete (preserve history kalau ada redemption sebelumnya).
CREATE TABLE redeemable_items (
  id          VARCHAR(36) PRIMARY KEY,
  name        VARCHAR(200) NOT NULL,
  description VARCHAR(500) NULL,
  image       VARCHAR(500) NULL,
  points_cost INT NOT NULL,
  stock       INT NOT NULL DEFAULT 0,
  redeemed    INT NOT NULL DEFAULT 0,
  is_active   TINYINT(1) NOT NULL DEFAULT 1,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at  TIMESTAMP NULL,
  INDEX idx_active (is_active, deleted_at)
);
