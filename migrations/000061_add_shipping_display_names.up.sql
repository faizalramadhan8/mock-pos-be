-- 28 Jul 2026 — pisahkan CODE dari DISPLAY untuk shipping courier/service.
-- Sebelum: shipping_courier = "SiCepat" (display), broken untuk pass ke Biteship API.
-- Sesudah: shipping_courier = "sicepat" (code), shipping_courier_name = "SiCepat" (display).
-- Backfill: kolom lama = display → move ke *_name; kolom courier/service akan
-- di-overwrite pakai code oleh order berikutnya via FE fix. Order lama tetap
-- punya display di kolom baru + code raw di lama (best-effort). Buat backfill
-- pakai heuristic mapping display → code.

ALTER TABLE orders
    ADD COLUMN shipping_courier_name VARCHAR(100) NULL AFTER shipping_courier,
    ADD COLUMN shipping_service_name VARCHAR(100) NULL AFTER shipping_service;

-- Backfill: kolom lama (yang menyimpan display) → kolom baru *_name.
-- Order ecom saja (POS tidak pakai kolom ini).
UPDATE orders
SET shipping_courier_name = shipping_courier,
    shipping_service_name = shipping_service
WHERE order_source = 'ecom'
  AND (shipping_courier_name IS NULL OR shipping_courier_name = '');

-- Kemudian lowercase + strip untuk kolom lama (jadi "code-ish") — best-effort.
-- Order lama yang gagal jadi code valid Biteship: admin harus edit manual atau
-- customer buat order baru.
UPDATE orders
SET shipping_courier =
    CASE
        WHEN LOWER(shipping_courier) LIKE '%j&t%' THEN 'jnt'
        WHEN LOWER(shipping_courier) LIKE '%jnt%' THEN 'jnt'
        WHEN LOWER(shipping_courier) LIKE '%ninja%' THEN 'ninja'
        WHEN LOWER(shipping_courier) LIKE '%pos indonesia%' THEN 'pos'
        WHEN LOWER(shipping_courier) LIKE '%sicepat%' THEN 'sicepat'
        WHEN LOWER(shipping_courier) LIKE '%jne%' THEN 'jne'
        WHEN LOWER(shipping_courier) LIKE '%anteraja%' THEN 'anteraja'
        WHEN LOWER(shipping_courier) LIKE '%tiki%' THEN 'tiki'
        ELSE LOWER(REPLACE(shipping_courier, ' ', ''))
    END,
    shipping_service =
    CASE
        WHEN LOWER(shipping_service) LIKE '%reguler%' THEN 'reg'
        WHEN LOWER(shipping_service) LIKE '%regular%' THEN 'reg'
        WHEN LOWER(shipping_service) LIKE '%yes%' THEN 'yes'
        WHEN LOWER(shipping_service) LIKE '%next day%' THEN 'yes'
        WHEN LOWER(shipping_service) LIKE '%besok sampai%' THEN 'bosstuj'
        WHEN LOWER(shipping_service) LIKE '%oke%' THEN 'oke'
        WHEN LOWER(shipping_service) LIKE '%eco%' THEN 'ecopack'
        WHEN LOWER(shipping_service) LIKE '%express%' THEN 'express'
        ELSE LOWER(REPLACE(shipping_service, ' ', ''))
    END
WHERE order_source = 'ecom';
