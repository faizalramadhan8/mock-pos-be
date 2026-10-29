-- Warna merek POS bisa diganti dari Pengaturan (29 Okt 2026).
--
-- Bu Santi: "Disaya ganti warna juga biru ya" — setelah meminta hal yang
-- sama untuk storefront. Disimpan di server, bukan di browser, supaya
-- tablet kasir dan laptopnya menampilkan warna yang sama.
--
-- Menyimpan NAMA palet, bukan kode warnanya; daftarnya ada di index.css POS.
-- Kosong/NULL = merah muda bawaan.
ALTER TABLE settings
  ADD COLUMN brand_color VARCHAR(20) NULL AFTER store_phone;
