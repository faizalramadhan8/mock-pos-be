DROP INDEX idx_orders_payments_edited_at ON orders;
ALTER TABLE orders
  DROP COLUMN payments_edited_reason,
  DROP COLUMN payments_edited_by,
  DROP COLUMN payments_edited_at;
