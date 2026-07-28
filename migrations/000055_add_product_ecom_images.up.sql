-- Support gallery multi-gambar untuk produk di storefront ecom.
-- Format: JSON array of URL strings, mis. ["/storage/products/abc.jpg", "/storage/products/def.jpg"].
-- ecom_image existing tetap dipakai sebagai gambar utama (thumbnail).
-- ecom_images = gambar tambahan untuk gallery swipe di halaman produk detail.
-- Kalau kosong = fallback tampil ecom_image saja (atau products.image POS).
ALTER TABLE products
  ADD COLUMN ecom_images JSON NULL AFTER ecom_image;
