-- Extend order_items untuk support redeem dari redeemable_items table.
-- Sebelum migration ini: tebus poin pakai produk dari katalog POS biasa
-- (product_id set + redeemed_with_points=true + unit_price = sellingPrice).
-- Sesudah: tebus poin dari katalog terpisah pakai redeemable_item_id.
--
-- Untuk redeem row baru (post migration):
--   product_id = ""  (empty string — sentinel, BE detect via RedeemableItemID != nil)
--   redeemable_item_id = <id>
--   unit_price = points_cost (interpret 1 poin = Rp 1 untuk konsisten dengan
--     existing applyPointsChange yang baca lineTotal = unit_price × qty)
--   redeemed_with_points = true
--
-- Tidak modify product_id ke NULL — empty string lebih simpler (Go entity
-- pakai `string` type, kompatibel dengan semua existing query). ON DELETE
-- SET NULL pada redeemable_item_id — kalau item dihapus, history aman.
ALTER TABLE order_items
  ADD COLUMN redeemable_item_id VARCHAR(36) NULL,
  ADD CONSTRAINT fk_order_items_redeemable_id
    FOREIGN KEY (redeemable_item_id) REFERENCES redeemable_items(id) ON DELETE SET NULL;

CREATE INDEX idx_order_items_redeemable_id ON order_items(redeemable_item_id);
