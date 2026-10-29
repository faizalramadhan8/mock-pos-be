-- Warna merek storefront bisa diganti dari Pengaturan (29 Okt 2026).
--
-- Bu Santi: "kalau bisa berikan pilihan warna beberapa jadi kita bisa ganti
-- manual sesuai warna yang tersedia", dan untuk toko rumah dia memilih biru
-- "seperti di kasir toko".
--
-- Menyimpan NAMA palet, bukan kode warnanya. Tiap palet butuh dua belas
-- nada yang serasi (sembilan cherry + tiga ink); membiarkan admin memilih
-- satu warna bebas akan menghasilkan kombinasi yang kontrasnya gagal dan
-- tulisannya tidak terbaca. Daftar paletnya ada di index.css storefront.
--
-- Kosong/NULL = merah muda bawaan.
ALTER TABLE ecom_settings
  ADD COLUMN brand_color VARCHAR(20) NULL AFTER store_logo;
