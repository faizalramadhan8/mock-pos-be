-- Koordinat titik jemput untuk kurir instan (Gojek/Grab/Borzo/Lalamove).
--
-- Fai 12 Okt 2026: "gojek dan grab sudah aktif di mode testing". Benar — tapi
-- mereka tidak pernah muncul di pilihan ongkir, dan penyebabnya bukan aktivasi.
--
-- Kurir instan menghitung tarif dari JARAK TEMPUH, bukan dari wilayah. Tanpa
-- koordinat mereka tidak bisa memberi harga sama sekali. Diuji pada rute
-- Cilangkap → Mekar Jaya (±7 km):
--   area_id saja        → 4 kurir  (jne, jnt, sicepat, anteraja)
--   koordinat saja      → 10 kurir (+ gojek, grab, borzo, lalamove, paxel, …)
--   area_id + koordinat → dua-duanya muncul
--
-- Koordinat TUJUAN sudah lama ada di `ecom_addresses` (diisi dari GPS customer).
-- Yang hilang cuma koordinat ASAL — toko/rumah Bu Santi. Itu yang ditambahkan
-- di sini, sebagai pengaturan yang bisa dia isi sendiri tanpa deploy.
ALTER TABLE ecom_settings
  ADD COLUMN store_pickup_latitude  DECIMAL(10,7) NULL AFTER store_pickup_area_id,
  ADD COLUMN store_pickup_longitude DECIMAL(10,7) NULL AFTER store_pickup_latitude;
