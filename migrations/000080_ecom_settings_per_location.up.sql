-- ecom_settings: dari satu baris jadi satu baris per tempat.
--
-- Sampai sekarang tabel ini singleton `id='default'` — satu nama toko, satu
-- logo, satu nomor WA, satu alamat jemput, satu warna merek. Dua storefront
-- tidak bisa punya identitas berbeda dari situ, padahal justru identitas
-- berbeda itu intinya (Tobaku Depok vs TBK Santi Bekasi).
--
-- Yang paling berbahaya alamat jemputnya: kurir akan dikirim ke satu alamat
-- untuk kedua toko, dan ongkir dihitung dari titik yang salah untuk salah
-- satunya — tanpa pesan salah apa pun.
--
-- Baris lama jadi milik RUMAH: storefront hari ini menyaring produk
-- location='home' dan rekening 'home', jadi itulah yang sebenarnya dilayani
-- baris 'default' selama ini.
ALTER TABLE ecom_settings
  ADD COLUMN location VARCHAR(10) NOT NULL DEFAULT 'home' AFTER id;

UPDATE ecom_settings SET location = 'home' WHERE id = 'default';

CREATE UNIQUE INDEX idx_ecom_settings_location ON ecom_settings(location);

-- Baris Toko Bekasi. Saklar pembayaran disalin dari baris rumah supaya
-- mematikan PG tetap berlaku untuk dua-duanya; alamat jemput, koordinat,
-- logo, dan warna SENGAJA dibiarkan kosong — diisi Bu Santi sendiri.
--
-- Alamat jemput kosong berarti titik asal jatuh ke variabel lingkungan
-- BITESHIP_ORIGIN_* yang isinya alamat rumah. Panel admin memperingatkan
-- ini dengan jelas; jangan menyalin alamat rumah ke sini sebagai "sementara",
-- karena ongkir yang dihitung dari alamat salah tidak pernah terlihat salah.
INSERT INTO ecom_settings (id, location, store_name, min_order_amount,
                           payment_pg_enabled, payment_manual_enabled,
                           notif_order_email_enabled)
SELECT 'store', 'store', 'Toko Bahan Kue Santi', 0,
       payment_pg_enabled, payment_manual_enabled, notif_order_email_enabled
FROM ecom_settings WHERE id = 'default'
ON DUPLICATE KEY UPDATE id = id;
