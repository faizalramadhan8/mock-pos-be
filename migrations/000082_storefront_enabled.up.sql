-- Saklar "toko ini tayang di storefront".
--
-- Tanpa ini, begitu penyaring `location='home'` dilepas dari katalog publik,
-- SELURUH produk Bekasi yang `ecom_is_available=1` langsung muncul di
-- storefront — ratusan barang, dengan ongkir dihitung dari alamat jemput yang
-- belum diisi (jatuh ke variabel lingkungan, yaitu alamat rumah). Salah
-- tanpa satu pun pesan salah.
--
-- Bawaan 0 = tidak tayang. Baris rumah dinyalakan di bawah karena memang
-- sudah melayani customer hari ini. Bekasi dinyalakan Bu Santi sendiri di
-- panel admin setelah alamat jemputnya terisi — rencananya habis Lebaran.
ALTER TABLE ecom_settings
  ADD COLUMN storefront_enabled TINYINT(1) NOT NULL DEFAULT 0 AFTER location;

UPDATE ecom_settings SET storefront_enabled = 1 WHERE location = 'home';
