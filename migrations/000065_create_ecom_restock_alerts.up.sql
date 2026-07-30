-- 30 Jul 2026 — Sprint 3 #16: customer subscribe alert produk restock.
-- Kalau produk yang habis / low stock kembali tersedia, customer dapet email
-- + push notification.
CREATE TABLE ecom_restock_alerts (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    product_id VARCHAR(36) NOT NULL,
    notified_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT current_timestamp(),
    UNIQUE KEY uniq_user_product (user_id, product_id),
    INDEX idx_alert_product (product_id, notified_at),
    CONSTRAINT fk_alert_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_alert_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);
