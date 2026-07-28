-- Add ecom_delivered_at untuk track kapan kurir tandai barang sampai.
-- Dipakai untuk:
--   1. Timeline "sudah sampai — konfirmasi ya" di customer app
--   2. Auto-complete cron 7 hari kalau customer lupa konfirmasi
-- NULL untuk order yang belum sampai atau order POS (bukan ecom).
ALTER TABLE orders
    ADD COLUMN ecom_delivered_at DATETIME NULL AFTER ecom_status;
