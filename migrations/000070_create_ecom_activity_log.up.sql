-- 2 Aug 2026 — Sprint 5 Chunk 9: Activity log admin ecom.
-- Audit trail semua aksi admin di storefront panel. Independent dari POS
-- audit_entries (scope + role beda). Cegah dispute "siapa ubah harga produk
-- X kemarin", "siapa yang broadcast promo salah alamat".
CREATE TABLE ecom_activity_log (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    admin_id VARCHAR(36) NOT NULL,
    -- Action slug — enum-like tapi VARCHAR biar fleksibel tambah tanpa migration.
    -- Contoh: product_published, product_unpublished, product_price_changed,
    -- refund_created, complaint_replied, settings_changed, broadcast_sent,
    -- customer_blocked, customer_unblocked, voucher_created, voucher_deleted,
    -- category_created, category_deleted, review_hidden, review_shown.
    action VARCHAR(50) NOT NULL,
    -- Target = entity yang dimutate. Contoh: "product:abc-123", "order:xyz",
    -- "settings", "voucher:v1". Format `type:id` opsional (bisa `settings` polos).
    target VARCHAR(100) NULL,
    -- Human-readable description. Fungsi FE tidak perlu translate.
    description VARCHAR(500) NOT NULL,
    -- Optional JSON snapshot (before/after) untuk aksi critical seperti price change.
    -- Skala kecil, JSON kolom OK. Full change tracking pakai product_price_history table.
    meta JSON NULL,
    created_at DATETIME NOT NULL DEFAULT current_timestamp(),
    INDEX idx_activity_created (created_at DESC),
    INDEX idx_activity_admin (admin_id, created_at DESC),
    INDEX idx_activity_action (action, created_at DESC),
    CONSTRAINT fk_activity_admin FOREIGN KEY (admin_id) REFERENCES users(id) ON DELETE RESTRICT
);
