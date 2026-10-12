-- Jatah stok online per produk tidak bisa dipulihkan otomatis — migrasi
-- menolkan `stock_ecom` dan tidak ada tempat lain yang menyimpannya. Kalau
-- memang perlu dipisah lagi, jatahnya harus di-set ulang manual oleh Bu Santi.
--
-- `stock` sengaja TIDAK disentuh di sini, sama seperti di up.sql: kolom itu
-- selalu berisi jumlah fisik sebenarnya, sebelum maupun sesudah penggabungan.

DROP INDEX idx_products_ecom_avail_stock ON products;
