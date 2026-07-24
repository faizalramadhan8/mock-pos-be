ALTER TABLE products DROP FOREIGN KEY fk_products_ecom_category;
ALTER TABLE products DROP INDEX idx_products_ecom_category;
ALTER TABLE products DROP COLUMN ecom_category_id;
DROP TABLE IF EXISTS ecom_categories;
