ALTER TABLE products
    ADD COLUMN supplier_id VARCHAR(36) NULL AFTER category_id,
    ADD KEY idx_products_supplier (supplier_id),
    ADD CONSTRAINT fk_products_supplier
        FOREIGN KEY (supplier_id) REFERENCES suppliers(id)
        ON DELETE SET NULL ON UPDATE CASCADE;
