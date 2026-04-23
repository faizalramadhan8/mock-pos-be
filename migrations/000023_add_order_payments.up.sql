CREATE TABLE IF NOT EXISTS order_payments (
    id         VARCHAR(36) NOT NULL PRIMARY KEY,
    order_id   VARCHAR(36) NOT NULL,
    method     VARCHAR(20) NOT NULL,
    amount     DECIMAL(15,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP   NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_order_payments_order (order_id),
    CONSTRAINT fk_order_payments_order
        FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);

-- Backfill for existing completed orders so they have at least one payment row
-- (method + full amount). Ignored if the table is empty.
INSERT INTO order_payments (id, order_id, method, amount, created_at)
SELECT
    CONCAT('bf-', o.id),
    o.id,
    o.payment,
    o.total,
    COALESCE(o.created_at, NOW())
FROM orders o
LEFT JOIN order_payments op ON op.order_id = o.id
WHERE op.id IS NULL AND o.status = 'completed';
