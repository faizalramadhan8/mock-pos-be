-- Biteship Order API integration (Sprint 3, Bu Santi 28 Jul 2026).
-- Simpan id order Biteship supaya:
--   1. Bisa track status via webhook (Biteship kirim `order_id` di payload)
--   2. Bisa cancel order lewat DELETE /v1/orders/:id
--   3. Bisa refetch info kalau webhook missed (query Biteship langsung)
-- Nullable — order lama pre-Sprint 3 + order manual resi tetap valid (NULL).
ALTER TABLE orders
  ADD COLUMN biteship_order_id VARCHAR(64) NULL AFTER shipping_awb,
  ADD INDEX idx_orders_biteship_id (biteship_order_id);
