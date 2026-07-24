-- Ecom-specific category taxonomy — terpisah dari POS `categories`.
-- Alasan: struktur belanja online beda dari sistem POS. POS categories dipakai
-- untuk reporting internal (mis. "Cokelat", "Butter"), sedangkan ecom
-- categories = customer-facing groupings ("Tepung & Bahan Kering",
-- "Cokelat & Kakao", "Toping & Hiasan"). Admin ecom curate independen.
CREATE TABLE IF NOT EXISTS ecom_categories (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    name_id VARCHAR(200) NULL,
    icon_emoji VARCHAR(16) NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_active TINYINT(1) NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    INDEX idx_ecom_cat_active_sort (deleted_at, is_active, sort_order),
    INDEX idx_ecom_cat_name (name)
);

-- Products link ke ecom category (opsional — kalau NULL storefront tetap
-- tampil produk tapi tidak ke-group di category browse).
ALTER TABLE products
    ADD COLUMN ecom_category_id VARCHAR(36) NULL AFTER ecom_image,
    ADD INDEX idx_products_ecom_category (ecom_category_id),
    ADD CONSTRAINT fk_products_ecom_category
        FOREIGN KEY (ecom_category_id) REFERENCES ecom_categories(id)
        ON DELETE SET NULL;
