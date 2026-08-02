-- 31 Jul 2026 — Sprint 4 Chunk 2: refund tracking untuk komplain approved.
-- Manual flow: Bu Santi transfer dana ke customer bank/e-wallet, catat di
-- sini + pilih item yang direstok. Tidak call DOKU refund API (scale kecil
-- + payment gateway refund complex, out-of-band lebih fleksibel).
CREATE TABLE ecom_refunds (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    order_id VARCHAR(36) NOT NULL,
    -- Optional link ke complaint. Refund bisa initiated tanpa complaint (rare,
    -- mis. admin proaktif refund karena masalah system).
    complaint_id VARCHAR(36) NULL,
    -- Amount refunded (partial atau full — bisa < order.total).
    amount DECIMAL(15,2) NOT NULL,
    -- Method: transfer_bank / ewallet / cash / voucher / other
    method VARCHAR(20) NOT NULL,
    -- Free-text: nomor rekening tujuan, bukti transfer URL, dll.
    note TEXT NULL,
    -- Snapshot item yang direstok (kalau ada). JSON array of {product_id, qty}.
    -- Kosong = customer keep barang (mis. refund partial diskon).
    restocked_items JSON NULL,
    -- Actor + timing
    refunded_by VARCHAR(36) NOT NULL, -- admin user_id
    refunded_at DATETIME NOT NULL DEFAULT current_timestamp(),
    created_at DATETIME NOT NULL DEFAULT current_timestamp(),
    INDEX idx_refund_order (order_id),
    INDEX idx_refund_complaint (complaint_id),
    INDEX idx_refund_created (created_at DESC),
    CONSTRAINT fk_refund_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    CONSTRAINT fk_refund_complaint FOREIGN KEY (complaint_id) REFERENCES ecom_complaints(id) ON DELETE SET NULL,
    CONSTRAINT fk_refund_by FOREIGN KEY (refunded_by) REFERENCES users(id) ON DELETE RESTRICT
);
