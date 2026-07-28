-- 29 Jul 2026 — simpan Biteship area_id di ecom_addresses supaya kurir yang
-- butuh area_id (Anteraja, Ninja, ID Express) bisa quote rate akurat.
-- Postal code sering ambigu (multi-kelurahan) — area_id resolve ke lokasi
-- spesifik. Nullable — order lama tanpa area_id tetap jalan via postal fallback.
ALTER TABLE ecom_addresses
    ADD COLUMN biteship_area_id VARCHAR(64) NULL AFTER zipcode;
