-- Payment gateway switch: Midtrans Snap → PG DOKU wrapper (28 Jul 2026).
-- Simpan payment_url dari PG (customer buka link ini untuk bayar) + channel
-- + category. payment_snap_token existing di-deprecate untuk order ecom baru
-- tapi tetap dipertahankan untuk compat dengan order lama yang pakai Midtrans.
ALTER TABLE orders
    ADD COLUMN payment_url VARCHAR(500) NULL AFTER payment_snap_token,
    ADD COLUMN payment_channel VARCHAR(20) NULL AFTER payment_url,
    ADD COLUMN payment_channel_category VARCHAR(30) NULL AFTER payment_channel;
