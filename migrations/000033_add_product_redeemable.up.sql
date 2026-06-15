-- Katalog Tebus Poin: admin curate daftar produk yang eligible untuk
-- ditebus pakai member.points. Default false — semua produk existing
-- tidak eligible sampai admin tandai eksplisit. Poin cost = selling_price
-- (1 poin = Rp 1), tidak ada custom cost field.
ALTER TABLE products ADD COLUMN is_redeemable BOOLEAN NOT NULL DEFAULT FALSE;
