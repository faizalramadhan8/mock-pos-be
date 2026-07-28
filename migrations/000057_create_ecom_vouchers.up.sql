-- Voucher/promo code system untuk storefront ecom.
-- Sprint 5, Bu Santi 28 Jul 2026.
--
-- Design decisions:
--   - Kode voucher case-insensitive (di-uppercase saat store + compare).
--   - Type: 'percent' (10 = 10%) atau 'fixed' (10000 = Rp 10.000 off).
--   - min_subtotal: minimum belanja untuk pakai voucher (0 = tidak ada minimum).
--   - max_discount: cap untuk type='percent' (mis. 20% but max Rp 50k).
--   - usage_limit: berapa kali voucher bisa dipakai global (0 = unlimited).
--   - used_count: counter increment tiap pakai. Cegah overuse.
--   - starts_at / expires_at: window valid (NULL = tidak dibatasi).
--   - is_active: kill switch admin (bisa disable tanpa hapus).
CREATE TABLE IF NOT EXISTS ecom_vouchers (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    code VARCHAR(50) NOT NULL,
    description VARCHAR(200) NULL,
    type VARCHAR(10) NOT NULL, -- 'percent' | 'fixed'
    value DECIMAL(15,2) NOT NULL,
    min_subtotal DECIMAL(15,2) NOT NULL DEFAULT 0,
    max_discount DECIMAL(15,2) NULL,
    usage_limit INT NOT NULL DEFAULT 0,
    used_count INT NOT NULL DEFAULT 0,
    starts_at DATETIME NULL,
    expires_at DATETIME NULL,
    is_active TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    UNIQUE INDEX uq_voucher_code (code),
    INDEX idx_vouchers_active (deleted_at, is_active, expires_at)
);

-- Kolom di orders untuk simpan snapshot voucher yang dipakai.
-- Simpan value + amount supaya historical akurat kalau voucher config berubah.
ALTER TABLE orders
  ADD COLUMN voucher_code VARCHAR(50) NULL AFTER shipping_cost,
  ADD COLUMN voucher_discount DECIMAL(15,2) NOT NULL DEFAULT 0 AFTER voucher_code,
  ADD INDEX idx_orders_voucher (voucher_code);
