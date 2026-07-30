-- 30 Jul 2026 — Sprint 1 #4: customer bisa ajukan komplain kalau barang
-- rusak/salah kirim/kurang. Cegah customer harus WA Bu Santi manual.
-- Bu Santi lihat + reply via Admin panel.
CREATE TABLE ecom_complaints (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    -- Reason enum: barang_rusak / barang_salah / barang_kurang / lainnya
    reason VARCHAR(30) NOT NULL,
    description TEXT NOT NULL,
    -- Photo evidence — array URL image, JSON. Max 5 (enforce di FE + BE).
    images JSON NULL,
    -- Status: open (baru submit) / in_review (admin lihat) / resolved / rejected
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    admin_reply TEXT NULL,
    admin_id VARCHAR(36) NULL,
    created_at DATETIME NOT NULL DEFAULT current_timestamp(),
    updated_at DATETIME NOT NULL DEFAULT current_timestamp() ON UPDATE current_timestamp(),
    resolved_at DATETIME NULL,
    INDEX idx_complaint_order (order_id),
    INDEX idx_complaint_user (user_id),
    INDEX idx_complaint_status (status, created_at DESC),
    CONSTRAINT fk_complaint_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    CONSTRAINT fk_complaint_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
