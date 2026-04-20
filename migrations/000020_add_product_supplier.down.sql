ALTER TABLE products
    DROP FOREIGN KEY fk_products_supplier,
    DROP KEY idx_products_supplier,
    DROP COLUMN supplier_id;
