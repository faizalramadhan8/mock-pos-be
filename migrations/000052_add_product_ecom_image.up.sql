-- Ecom-specific product image (terpisah dari products.image yang dipakai POS).
-- Admin ecom bisa upload foto khusus untuk storefront (mis. lifestyle shot),
-- tapi kalau kosong FE fallback ke products.image.
ALTER TABLE products
  ADD COLUMN ecom_image VARCHAR(500) NULL AFTER ecom_description;
