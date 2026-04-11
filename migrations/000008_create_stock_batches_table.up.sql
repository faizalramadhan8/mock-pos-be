CREATE TABLE stock_batches (
    id VARCHAR(36) NOT NULL,
    product_id VARCHAR(36) NOT NULL,
    quantity INT NOT NULL DEFAULT 0,
    expiry_date DATE NULL,
    received_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    note TEXT NULL,
    batch_number VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_stock_batches_product_id (product_id),
    KEY idx_stock_batches_expiry_date (expiry_date),
    CONSTRAINT fk_stock_batches_product FOREIGN KEY (product_id) REFERENCES products(id)
);
