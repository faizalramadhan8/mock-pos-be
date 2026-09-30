-- CATATAN: penggabungan angkanya TIDAK bisa dibalik otomatis. Setelah
-- stock += stock_ecom, tidak ada jejak berapa yang dulu dijatah untuk online.
-- Down migration ini hanya membuang index baru; kalau memang perlu memisah
-- lagi, alokasi stock_ecom harus di-set ulang manual oleh Bu Santi.

DROP INDEX idx_products_ecom_avail_stock ON products;
