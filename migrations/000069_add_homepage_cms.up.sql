-- 2 Aug 2026 — Sprint 5 Chunk 7: Homepage CMS.
-- Extend ecom_settings dengan field hero + pinned products supaya admin bisa
-- edit tampilan Home page tanpa deploy. Simpler dari bikin tabel baru.
ALTER TABLE ecom_settings
    ADD COLUMN hero_kicker    VARCHAR(80)  NULL AFTER notif_order_email_enabled,
    ADD COLUMN hero_title     VARCHAR(200) NULL AFTER hero_kicker,
    ADD COLUMN hero_subtitle  VARCHAR(300) NULL AFTER hero_title,
    ADD COLUMN hero_cta_label VARCHAR(50)  NULL AFTER hero_subtitle,
    ADD COLUMN hero_cta_url   VARCHAR(200) NULL AFTER hero_cta_label,
    -- JSON array of product IDs (max ~20). Rendered di section "Pilihan Bu Santi"
    -- di Home page, override "featured" default (yang ambil 8 produk latest).
    ADD COLUMN pinned_product_ids JSON NULL AFTER hero_cta_url,
    -- JSON array of category IDs untuk "Kategori Unggulan" section. Kalau
    -- kosong, semua kategori aktif ditampilkan (behavior lama).
    ADD COLUMN featured_category_ids JSON NULL AFTER pinned_product_ids;
