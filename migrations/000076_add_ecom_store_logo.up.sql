-- Logo storefront jadi isian, bukan gambar yang ditulis di kode.
--
-- Bu Santi punya logo untuk toko rumah (Tobaku — Toko Bahan Kue Kaya
-- Bersama), "tapi bisa digonta ganti juga". Sampai sekarang logonya adalah
-- SVG yang ditulis langsung di BakeryLogo.tsx, jadi tiap pergantian menuntut
-- ubah kode + deploy.
--
-- Menyimpan URL, bukan berkasnya. Gambarnya lewat endpoint upload yang sudah
-- ada (`POST /upload?type=store-logo` → /storage/...), sama seperti foto
-- produk dan bukti transfer.
ALTER TABLE ecom_settings
  ADD COLUMN store_logo VARCHAR(500) NULL AFTER store_name;
