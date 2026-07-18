-- Track edit metode pembayaran setelah checkout (Bu Santi 12 Jul 2026).
-- Skenario: kasir salah pilih Transfer padahal QRIS → admin/superadmin fix
-- via PATCH /orders/:id/payments. 3 kolom untuk audit inline:
--   payments_edited_at    : kapan diubah (NULL = never edited)
--   payments_edited_by    : user id yang ubah
--   payments_edited_reason: alasan mandatory (mis. "Kasir salah input")
-- Detail action tetap ke audit_log table untuk full trail.
ALTER TABLE orders
  ADD COLUMN payments_edited_at DATETIME NULL AFTER payment,
  ADD COLUMN payments_edited_by VARCHAR(36) NULL AFTER payments_edited_at,
  ADD COLUMN payments_edited_reason VARCHAR(500) NULL AFTER payments_edited_by;

CREATE INDEX idx_orders_payments_edited_at ON orders(payments_edited_at);
