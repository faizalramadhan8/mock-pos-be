-- 2 Aug 2026 — Sprint 5 Chunk 6: broadcast push admin panel.
-- History log tiap broadcast dikirim admin. Cegah admin spam tanpa audit +
-- kasih metrik keberhasilan (delivered vs failed).
CREATE TABLE ecom_broadcasts (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    title VARCHAR(100) NOT NULL,
    body VARCHAR(500) NOT NULL,
    -- URL yang customer buka saat klik notif. Bisa relatif (/produk/xxx) atau full.
    url VARCHAR(500) NULL,
    -- Snapshot metrik saat send. Delivered = sub yang HTTP 2xx dari push service.
    delivered_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    -- Sub count total di saat broadcast (audit: "berapa reach potential").
    total_subscribers INT NOT NULL DEFAULT 0,
    sent_by VARCHAR(36) NOT NULL,
    sent_at DATETIME NOT NULL DEFAULT current_timestamp(),
    INDEX idx_broadcast_sent (sent_at DESC),
    CONSTRAINT fk_broadcast_sender FOREIGN KEY (sent_by) REFERENCES users(id) ON DELETE RESTRICT
);
