-- Gabung stok toko dan stok e-commerce jadi satu pool.
--
-- Fai + Bu Santi 17 Sep 2026: "stok online dan stok e-commerce jadi 1".
-- Sebelumnya storefront memakai `products.stock_ecom` dan POS memakai
-- `products.stock`. Bu Santi harus mengisi jatah online per produk dan sering
-- lupa, akibatnya storefront bilang habis padahal barangnya ada di rak.
--
-- ╔══════════════════════════════════════════════════════════════════════════╗
-- ║ `stock_ecom` BUKAN TUMPUKAN BARANG TERPISAH — JANGAN DITAMBAHKAN.        ║
-- ╚══════════════════════════════════════════════════════════════════════════╝
--
-- Versi pertama migrasi ini menjalankan `UPDATE products SET stock = stock +
-- stock_ecom` dan itu SALAH. `stock_ecom` adalah **jatah dari stok yang sama**:
-- "dari barang yang ada di rak, maksimal sekian boleh dijual online". Bukan
-- unit tambahan. Buktinya ada di panel admin ecom sendiri, yang dulu menulis
-- "Stok toko fisik saat ini: N (dikelola di POS, tidak di sini)" dan memberi
-- peringatan kalau "Stok online lebih besar dari stok toko" — peringatan yang
-- hanya masuk akal kalau stok online adalah bagian dari stok toko.
--
-- Akibat di produksi (1 Okt 2026, Bu Santi: "perubahan stok opname rak dan
-- komputer berubah"): 105 produk menggelembung total 914 unit. Rich Gold Label
-- 500 ml fisik 0 jadi 11 di sistem, Anchor Processed Cheddar fisik 1 jadi 11.
-- Produk yang jatah online-nya 0 tetap cocok — itu yang membuka kasusnya.
-- Dipulihkan dari backup `products` pra-migrasi dengan mengurangi kembali
-- `stock_ecom` lama, plus satu baris stock_movements per produk sebagai jejak.
--
-- Yang benar: `stock` TIDAK disentuh sama sekali. Ia sudah berisi jumlah fisik
-- sebenarnya sejak awal. Yang berubah cuma: semua pembacaan dan penulisan
-- storefront dialihkan dari `stock_ecom` ke `stock` (lihat perubahan di
-- ecom_checkout/ecom_orders/ecom_admin usecase), dan kolom lamanya dinolkan
-- supaya kalau ada kode yang terlewat dan masih menulis ke sana, efeknya
-- langsung kelihatan alih-alih diam-diam salah.

UPDATE products SET stock_ecom = 0 WHERE stock_ecom <> 0;

-- Storefront browse memfilter `ecom_is_available = 1 AND stock > 0`.
-- Index lama idx_products_ecom_avail(ecom_is_available, stock_ecom) sekarang
-- percuma karena kolomnya konstan 0 — dibiarkan (dipakai lagi kalau rollback).
CREATE INDEX idx_products_ecom_avail_stock ON products(ecom_is_available, stock);
