-- Snapshot paket_count + extra_count per order_item untuk reportable bundling.
-- Per request Bu Santi 21 Jun 2026 — laporan bundling perlu tau:
--   "berapa paket terjual" + "berapa extra (sisa modulo)" tanpa harus JOIN tier
--   (tier bisa dihapus → tier_id NULL → tidak bisa derive).
--
-- Konvensi:
--   paket_count = floor(qty_satuan / tier.min_qty)  [0 kalau bukan tier]
--   extra_count = qty_satuan % tier.min_qty         [0 kalau bukan tier]
--   qty_satuan = quantity × qty_per_box kalau unit_type='box', else quantity
--   total satuan terjual = paket_count × tier.min_qty + extra_count = qty_satuan
--
-- Default 0 supaya backfill aman (existing rows = 0 paket; price_source =
-- 'regular'/'member_price' tetap konsisten karena paket_count cuma relevan
-- saat price_source ∈ {tier_all, tier_member}).
ALTER TABLE order_items
  ADD COLUMN paket_count INT NOT NULL DEFAULT 0,
  ADD COLUMN extra_count INT NOT NULL DEFAULT 0;
