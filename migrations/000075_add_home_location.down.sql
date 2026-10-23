-- Rollback. Produk milik toko rumah ikut kehilangan penandanya dan akan
-- terbaca sebagai produk Bekasi — muncul di kasir. Hapus atau nonaktifkan
-- produk rumah dulu sebelum menjalankan ini.
DROP INDEX idx_products_loc_avail_stock ON products;
DROP INDEX idx_products_location ON products;
ALTER TABLE products DROP COLUMN location;

DROP INDEX idx_stock_movements_location ON stock_movements;
ALTER TABLE stock_movements DROP COLUMN location;

ALTER TABLE bank_accounts DROP COLUMN location;

UPDATE orders SET order_source = 'ecom' WHERE order_source = 'home';
