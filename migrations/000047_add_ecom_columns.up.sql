-- E-commerce v1 support (Bu Santi bahan kue online, 20 Jul 2026).
--
-- Channel separation: stok dan harga BEDA antara POS toko fisik vs storefront
-- online. Existing kolom `products.stock` + `products.selling_price` +
-- `products.member_price` = tetap = data POS. Kolom baru = data ecom.
--
-- Design decisions (per research + owner discussion):
--   - stock_ecom NULL vs 0: pakai default 0. Bu Santi harus explicit set
--     berapa yang di-allocate untuk online. Cegah "kok tiba-tiba tampil di
--     online padahal Bu Santi belum ready".
--   - ecom_price NULL = fallback ke selling_price (POS price). Kalau Bu Santi
--     mau harga sama offline + online, cukup biarkan NULL. Kalau beda, isi.
--   - ecom_is_available default TRUE: produk lama otomatis eligible tampil
--     kalau stock_ecom > 0. Kalau Bu Santi mau exclude specific SKU (mis.
--     grosir-only, atau produk sensitif), toggle FALSE.
--   - ecom_min_order default 1: MOQ per checkout. Bu Santi bisa set 2 kg
--     untuk tepung supaya tidak jual 200g via online (ongkir gak worth).
--   - ecom_weight_grams NULL = product tidak bisa di-order online sampai
--     weight di-set (shipping API butuh weight untuk rate calc).
--
-- Order source: tandai order dari mana. Existing orders backfill 'pos'.

ALTER TABLE products
  ADD COLUMN stock_ecom INT NOT NULL DEFAULT 0 AFTER stock,
  ADD COLUMN ecom_price DECIMAL(15,2) NULL AFTER selling_price,
  ADD COLUMN ecom_member_price DECIMAL(15,2) NULL AFTER member_price,
  ADD COLUMN ecom_is_available BOOLEAN NOT NULL DEFAULT TRUE,
  ADD COLUMN ecom_description TEXT NULL,
  ADD COLUMN ecom_weight_grams INT NULL,
  ADD COLUMN ecom_min_order INT NOT NULL DEFAULT 1;

-- Index untuk query "produk yang tampil di storefront":
--   WHERE ecom_is_available = 1 AND stock_ecom > 0
CREATE INDEX idx_products_ecom_avail ON products(ecom_is_available, stock_ecom);

ALTER TABLE orders
  ADD COLUMN order_source VARCHAR(10) NOT NULL DEFAULT 'pos' AFTER status;

-- Backfill existing orders = 'pos' (semua trx historical = POS toko fisik).
-- Default column value sudah handle ini, tapi explicit UPDATE untuk clarity.
UPDATE orders SET order_source = 'pos' WHERE order_source = 'pos' OR order_source = '';

CREATE INDEX idx_orders_source ON orders(order_source);
CREATE INDEX idx_orders_source_status ON orders(order_source, status);
