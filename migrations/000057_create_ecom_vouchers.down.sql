ALTER TABLE orders
  DROP INDEX idx_orders_voucher,
  DROP COLUMN voucher_discount,
  DROP COLUMN voucher_code;
DROP TABLE IF EXISTS ecom_vouchers;
