-- Rollback: swap kolom back — display names dari *_name kembali ke shipping_*.
UPDATE orders
SET shipping_courier = shipping_courier_name,
    shipping_service = shipping_service_name
WHERE order_source = 'ecom'
  AND shipping_courier_name IS NOT NULL;

ALTER TABLE orders
    DROP COLUMN shipping_service_name,
    DROP COLUMN shipping_courier_name;
