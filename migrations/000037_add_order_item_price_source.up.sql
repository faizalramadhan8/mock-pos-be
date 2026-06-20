-- Audit: tag setiap order_item dengan source harga supaya bisa report
-- "transaksi pakai harga grosir bulan ini" + drill ke tier mana yang dipakai.
--
-- price_source values:
--   'regular'      → sellingPrice (walk-in atau member tanpa member_price)
--   'member_price' → memberPrice baseline (member tanpa tier match)
--   'tier_all'     → tier dengan target_type='all_customers'
--   'tier_member'  → tier dengan target_type='member_specific'
--
-- tier_id nullable FK ke product_price_tiers. ON DELETE SET NULL supaya
-- delete tier tidak hilang order history-nya — history snapshot tetap aman,
-- cuma kehilangan link ke tier yang sudah tidak ada.
ALTER TABLE order_items
  ADD COLUMN price_source VARCHAR(20) NOT NULL DEFAULT 'regular',
  ADD COLUMN tier_id VARCHAR(36) NULL,
  ADD CONSTRAINT fk_order_items_tier_id
    FOREIGN KEY (tier_id) REFERENCES product_price_tiers(id) ON DELETE SET NULL;

CREATE INDEX idx_order_items_price_source ON order_items(price_source);
CREATE INDEX idx_order_items_tier_id ON order_items(tier_id);

-- Backfill heuristic untuk data historis:
--   unit_price < regular_price AND member_id IS NOT NULL → 'member_price' (tier_id NULL)
--   sisanya tetap default 'regular'
-- Tidak bisa derive 'tier_all'/'tier_member' karena tidak ada link historis.
UPDATE order_items oi
  JOIN orders o ON o.id = oi.order_id
  SET oi.price_source = 'member_price'
  WHERE o.member_id IS NOT NULL
    AND oi.regular_price IS NOT NULL
    AND oi.unit_price < oi.regular_price;
