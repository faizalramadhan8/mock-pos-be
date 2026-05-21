-- Tambah kategori belanja cash ke toko grosir (tidak ada faktur formal).
-- Owner pergi ke toko, bayar cash, bawa pulang barang — tidak track via
-- Faktur Barang Masuk karena tidak ada surat jalan/invoice formal.
--
-- 4 toko langganan: Yoeks, Indogrosir, Indoguna, Sukanda. is_system=1 supaya
-- tidak bisa dihapus dari UI (hanya di-deactivate). sort_order di range 15
-- (di antara "Gaji & Lemburan Pegawai"=10 dan "Plastik & Kemasan"=20) supaya
-- mudah dilihat di dropdown — pengeluaran rutin paling sering.

INSERT INTO expense_categories (id, name, is_system, sort_order) VALUES
  (UUID(), 'Belanja Yoeks',     1, 11),
  (UUID(), 'Belanja Indogrosir', 1, 12),
  (UUID(), 'Belanja Indoguna',   1, 13),
  (UUID(), 'Belanja Sukanda',    1, 14);
