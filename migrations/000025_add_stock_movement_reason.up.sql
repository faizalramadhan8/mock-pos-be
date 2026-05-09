-- Reason kategori untuk stock_movement, melengkapi `type` (in/out) supaya
-- audit trail lebih informatif. Contoh value: "restock", "sale", "repack",
-- "lost", "damaged", "opname", "cancel", "refund", "other".
-- Default empty string untuk record lama (pre-migration).
ALTER TABLE stock_movements
  ADD COLUMN reason VARCHAR(20) NOT NULL DEFAULT '' AFTER unit_price;

-- Backfill heuristik: tag movement lama berdasarkan note prefix.
UPDATE stock_movements SET reason = 'sale'
  WHERE type = 'out' AND note LIKE 'Sale%';

UPDATE stock_movements SET reason = 'restock'
  WHERE type = 'in' AND reason = '' AND supplier_id IS NOT NULL;
