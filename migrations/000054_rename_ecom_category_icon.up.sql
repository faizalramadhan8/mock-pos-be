-- Ganti icon_emoji (VARCHAR 16, emoji) → icon_name (VARCHAR 50, lucide icon
-- name). Bu Santi 24 Jul 2026 minta no emoji di FE — pakai lucide-react
-- (Wheat, Cookie, Milk, dst) yang konsisten cross-platform + scalable.
ALTER TABLE ecom_categories
    CHANGE COLUMN icon_emoji icon_name VARCHAR(50) NULL;
