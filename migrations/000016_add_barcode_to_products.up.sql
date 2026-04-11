ALTER TABLE products
    ADD COLUMN barcode VARCHAR(50) NULL AFTER sku,
    ADD KEY idx_products_barcode (barcode);
