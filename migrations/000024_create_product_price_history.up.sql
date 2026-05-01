CREATE TABLE IF NOT EXISTS product_price_history (
    id          VARCHAR(36)   NOT NULL PRIMARY KEY,
    product_id  VARCHAR(36)   NOT NULL,
    price_type  VARCHAR(20)   NOT NULL,
    price       DECIMAL(15,2) NOT NULL DEFAULT 0,
    status      VARCHAR(20)   NOT NULL DEFAULT 'active',
    start_date  TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    end_date    TIMESTAMP     NULL DEFAULT NULL,
    changed_by  VARCHAR(36)   NULL,
    note        VARCHAR(255)  NULL,
    created_at  TIMESTAMP     NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_pph_product       (product_id),
    KEY idx_pph_product_type  (product_id, price_type, status),
    KEY idx_pph_lookup_window (product_id, price_type, start_date, end_date),
    CONSTRAINT fk_pph_product
        FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);

-- Seed an active row per price_type for every existing product so historical
-- lookups never miss. start_date = product.created_at, end_date = NULL.
INSERT INTO product_price_history (id, product_id, price_type, price, status, start_date, end_date, created_at)
SELECT UUID(), p.id, 'regular',  p.selling_price,  'active', COALESCE(p.created_at, NOW()), NULL, NOW()
FROM products p
WHERE p.deleted_at IS NULL;

INSERT INTO product_price_history (id, product_id, price_type, price, status, start_date, end_date, created_at)
SELECT UUID(), p.id, 'purchase', p.purchase_price, 'active', COALESCE(p.created_at, NOW()), NULL, NOW()
FROM products p
WHERE p.deleted_at IS NULL;

INSERT INTO product_price_history (id, product_id, price_type, price, status, start_date, end_date, created_at)
SELECT UUID(), p.id, 'member',   p.member_price,   'active', COALESCE(p.created_at, NOW()), NULL, NOW()
FROM products p
WHERE p.deleted_at IS NULL AND p.member_price IS NOT NULL;
