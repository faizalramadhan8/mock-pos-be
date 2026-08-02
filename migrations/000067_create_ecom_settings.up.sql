-- 31 Jul 2026 — Sprint 4 Chunk 5: Ecom-specific settings.
-- Singleton row (id='default') — admin edit via /admin/pengaturan.
-- Terpisah dari POS `settings` supaya bisa berkembang independen.
CREATE TABLE ecom_settings (
    id VARCHAR(36) NOT NULL PRIMARY KEY DEFAULT 'default',

    -- Min order value (Rp). Checkout ditolak kalau subtotal < ini. 0 = tidak ada min.
    min_order_amount DECIMAL(15,2) NOT NULL DEFAULT 0,

    -- FloatingWA contact — nomor WA Bu Santi di storefront (E.164, mis. 6281574273040).
    wa_contact_number VARCHAR(20) NOT NULL DEFAULT '',
    wa_pretext TEXT NULL, -- default text saat customer klik floating WA

    -- Announcement bar — banner sticky top storefront (mis. "Gratis ongkir min 200k!").
    announcement_bar_enabled TINYINT(1) NOT NULL DEFAULT 0,
    announcement_bar_text VARCHAR(200) NULL,
    -- Optional CTA link + label (mis. "Lihat produk →" → /kategori/tepung)
    announcement_bar_cta_label VARCHAR(50) NULL,
    announcement_bar_cta_url VARCHAR(200) NULL,

    -- Store info yang dipakai untuk email confirmation + Biteship sender.
    store_name VARCHAR(200) NOT NULL DEFAULT 'Toko Bahan Kue Santi',
    store_email VARCHAR(200) NULL,
    store_pickup_address TEXT NULL,          -- alamat lengkap untuk kurir pickup
    store_pickup_phone VARCHAR(20) NULL,
    store_pickup_area_id VARCHAR(50) NULL,   -- Biteship area_id toko

    -- Toggle metode pembayaran (kalau owner mau matiin salah satu channel)
    payment_pg_enabled TINYINT(1) NOT NULL DEFAULT 1,      -- DOKU (VA/QRIS/e-wallet)
    payment_manual_enabled TINYINT(1) NOT NULL DEFAULT 1,  -- Transfer manual

    -- Toggle notifikasi
    notif_order_email_enabled TINYINT(1) NOT NULL DEFAULT 1,

    created_at DATETIME NOT NULL DEFAULT current_timestamp(),
    updated_at DATETIME NOT NULL DEFAULT current_timestamp() ON UPDATE current_timestamp()
);

-- Seed default singleton row.
INSERT INTO ecom_settings (id) VALUES ('default');
