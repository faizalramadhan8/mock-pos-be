-- Gabung stok toko dan stok e-commerce jadi satu pool.
--
-- Fai + Bu Santi 17 Sep 2026: "stok online dan stok e-commerce jadi 1".
-- Sebelumnya `products.stock` (POS) dan `products.stock_ecom` (storefront)
-- adalah dua angka terpisah — Bu Santi harus alokasi manual berapa yang
-- "dijatah" untuk online, dan sering lupa. Akibatnya storefront bilang habis
-- padahal barangnya menumpuk di toko, atau sebaliknya.
--
-- Setelah migration ini `products.stock` adalah SATU-SATUNYA angka stok.
-- Kolom `stock_ecom` sengaja TIDAK di-drop:
--   1. Rollback aman kalau ternyata ada write path yang kelewat (angkanya
--      tinggal dilihat, bukan hilang).
--   2. Beberapa response DTO masih memakai nama field `stock_ecom` — yang
--      berubah cuma sumbernya (sekarang dibaca dari `stock`).
-- Kolom di-set 0 supaya kalau ada kode lama yang lolos dan masih menulis ke
-- sana, efeknya kelihatan (stok online mendadak 0) bukan diam-diam salah.

UPDATE products SET stock = stock + stock_ecom WHERE stock_ecom > 0;
UPDATE products SET stock_ecom = 0 WHERE stock_ecom <> 0;

-- Storefront browse memfilter `ecom_is_available = 1 AND stock > 0`.
-- Index lama idx_products_ecom_avail(ecom_is_available, stock_ecom) sekarang
-- percuma karena kolomnya konstan 0 — dibiarkan (dipakai lagi kalau rollback).
CREATE INDEX idx_products_ecom_avail_stock ON products(ecom_is_available, stock);
