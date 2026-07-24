DROP INDEX idx_orders_payment_ref ON orders;
DROP INDEX idx_orders_ecom_status ON orders;
DROP INDEX idx_orders_ecom_user ON orders;

ALTER TABLE orders
  DROP COLUMN payment_expired_at,
  DROP COLUMN payment_paid_at,
  DROP COLUMN payment_reference,
  DROP COLUMN payment_snap_token,
  DROP COLUMN ecom_status,
  DROP COLUMN shipping_awb,
  DROP COLUMN shipping_etd,
  DROP COLUMN shipping_cost,
  DROP COLUMN shipping_service,
  DROP COLUMN shipping_courier,
  DROP COLUMN shipping_address_snapshot,
  DROP COLUMN ecom_user_id;
