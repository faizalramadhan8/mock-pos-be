-- 28 Jul 2026 — simpan Biteship tracking link + tracking_id.
-- tracking_url = public Biteship link untuk customer klik "Lacak Paket" tanpa
-- copy AWB ke web kurir manual. Hemat CS Bu Santi jawab "resi sudah dikirim
-- tapi kok belum update".
-- tracking_id = Biteship internal id, dipakai GET /v1/trackings/:id kalau
-- admin butuh sync manual (webhook missed).
ALTER TABLE orders
    ADD COLUMN shipping_tracking_url VARCHAR(500) NULL AFTER shipping_awb,
    ADD COLUMN shipping_tracking_id  VARCHAR(64)  NULL AFTER shipping_tracking_url;
