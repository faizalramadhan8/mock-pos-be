CREATE TABLE refunds (
    id VARCHAR(36) NOT NULL,
    order_id VARCHAR(36) NOT NULL,
    amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    reason TEXT NULL,
    created_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_refunds_order_id (order_id),
    CONSTRAINT fk_refunds_order FOREIGN KEY (order_id) REFERENCES orders(id)
);

CREATE TABLE refund_items (
    id VARCHAR(36) NOT NULL,
    refund_id VARCHAR(36) NOT NULL,
    product_id VARCHAR(36) NOT NULL,
    name VARCHAR(200) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    unit_type VARCHAR(20) NOT NULL DEFAULT 'individual',
    unit_price DECIMAL(15,2) NOT NULL DEFAULT 0,
    refund_amount DECIMAL(15,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_refund_items_refund_id (refund_id),
    CONSTRAINT fk_refund_items_refund FOREIGN KEY (refund_id) REFERENCES refunds(id) ON DELETE CASCADE
);
