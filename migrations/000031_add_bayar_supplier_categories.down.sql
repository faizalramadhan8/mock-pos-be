-- Rollback: hapus 8 kategori Bayar supplier, restore nama singkat Belanja,
-- restore sort_order Plastik & Kemasan.

DELETE FROM expense_categories
WHERE name IN (
  'Bayar Heri Jatinegara',
  'Bayar Mulia Raya',
  'Bayar Adyaceda Amandelis',
  'Bayar Howki',
  'Bayar Indoguna Utama',
  'Bayar Astaguna Wisesa',
  'Bayar Primarasa',
  'Bayar Tbk Kencana Makmur Bogor'
)
AND is_system = 1;

UPDATE expense_categories SET name = 'Belanja Indogrosir'
WHERE name = 'Belanja Indogrosir Bogor' AND is_system = 1;

UPDATE expense_categories SET name = 'Belanja Indoguna'
WHERE name = 'Belanja Indoguna Utama' AND is_system = 1;

UPDATE expense_categories SET name = 'Belanja Sukanda'
WHERE name = 'Belanja Sukanda Djaya' AND is_system = 1;

UPDATE expense_categories
SET sort_order = 20
WHERE name = 'Plastik & Kemasan' AND is_system = 1;
