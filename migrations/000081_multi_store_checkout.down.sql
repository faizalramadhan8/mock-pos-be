DROP INDEX idx_ecom_vouchers_location ON ecom_vouchers;
ALTER TABLE ecom_vouchers DROP COLUMN location;
DROP INDEX idx_orders_checkout_group ON orders;
ALTER TABLE orders DROP COLUMN checkout_group_id;
