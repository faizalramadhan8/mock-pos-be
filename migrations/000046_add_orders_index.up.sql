-- Perf fix (Bu Santi 12 Jul 2026: "sales lambat masuk").
-- Root cause query 370ms:
--   SELECT * FROM orders WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT 2000
-- MySQL kena filesort karena tidak ada index untuk (deleted_at, created_at).
-- Composite index bikin index range scan + no filesort. Ekspektasi drop
-- ke 30-50ms untuk 12k rows.
CREATE INDEX idx_orders_deleted_created ON orders(deleted_at, created_at DESC);

-- Products: query LIMIT tanpa filter juga sering ORDER BY created_at. Same fix.
CREATE INDEX idx_products_deleted_created ON products(deleted_at, created_at DESC);
