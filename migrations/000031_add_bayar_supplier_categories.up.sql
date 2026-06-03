-- Tambah kategori bayar supplier formal + rename existing Belanja categories
-- ke nama lengkap supplier (match dengan record di Stok > Pemasok). Owner
-- workflow baru: semua pembayaran ke supplier (cash atau tempo) dicatat via
-- Pengeluaran kategori "Bayar X" atau "Belanja X" — bukan via Tandai Lunas
-- di Faktur. Faktur tetap sebagai pure record (sudah disesuaikan sebelumnya).
--
-- Penamaan match supplier real (Stok > Pemasok list):
--   Adyaceda Amandelis, Astaguna Wisesa, Heri Jatinegara, Howki,
--   Indogrosir Bogor, Indoguna Utama, Mulia Raya, Primarasa, Sukanda Djaya,
--   Tbk Kencana Makmur Bogor, Yoeks.
--
-- Kelompok di expense_categories:
--   10  Gaji & Lemburan Pegawai
--   11-14 Belanja toko grosir (cash on-the-spot)
--   15-22 Bayar supplier formal (delivery + faktur)
--   25  Plastik & Kemasan (di-bump dari sort_order 20 supaya tidak collide)
--   30+ Utilities (Listrik, Air, dll)

-- Bump Plastik & Kemasan dari 20 → 25 supaya supplier categories bisa
-- duduk berurutan setelah Belanja toko (15-22).
UPDATE expense_categories
SET sort_order = 25
WHERE name = 'Plastik & Kemasan' AND is_system = 1;

-- Rename existing Belanja categories ke nama lengkap supplier.
UPDATE expense_categories SET name = 'Belanja Indogrosir Bogor'
WHERE name = 'Belanja Indogrosir' AND is_system = 1;

UPDATE expense_categories SET name = 'Belanja Indoguna Utama'
WHERE name = 'Belanja Indoguna' AND is_system = 1;

UPDATE expense_categories SET name = 'Belanja Sukanda Djaya'
WHERE name = 'Belanja Sukanda' AND is_system = 1;

-- Insert kategori bayar supplier formal — nama match supplier records.
INSERT INTO expense_categories (id, name, is_system, sort_order) VALUES
  (UUID(), 'Bayar Heri Jatinegara',           1, 15),
  (UUID(), 'Bayar Mulia Raya',                1, 16),
  (UUID(), 'Bayar Adyaceda Amandelis',        1, 17),
  (UUID(), 'Bayar Howki',                     1, 18),
  (UUID(), 'Bayar Indoguna Utama',            1, 19),
  (UUID(), 'Bayar Astaguna Wisesa',           1, 20),
  (UUID(), 'Bayar Primarasa',                 1, 21),
  (UUID(), 'Bayar Tbk Kencana Makmur Bogor',  1, 22);
