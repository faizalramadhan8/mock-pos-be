DROP INDEX idx_orders_source_status ON orders;
DROP INDEX idx_orders_source ON orders;
ALTER TABLE orders DROP COLUMN order_source;

DROP INDEX idx_products_ecom_avail ON products;
ALTER TABLE products
  DROP COLUMN ecom_min_order,
  DROP COLUMN ecom_weight_grams,
  DROP COLUMN ecom_description,
  DROP COLUMN ecom_is_available,
  DROP COLUMN ecom_member_price,
  DROP COLUMN ecom_price,
  DROP COLUMN stock_ecom;
