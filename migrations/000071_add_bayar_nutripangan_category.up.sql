-- 3 Aug 2026 — Bu Santi request: tambah kategori "Bayar Nutripangan" di
-- Pengeluaran. Nutripangan adalah supplier baru yang perlu di-track
-- pembayarannya via Pengeluaran (mirror pattern 000031_add_bayar_supplier).
--
-- Slot sort_order 23 — melanjutkan urutan supplier categories (15-22 sudah
-- diisi di 000031). Plastik & Kemasan tetap di sort_order 25.
INSERT INTO expense_categories (id, name, is_system, sort_order) VALUES
  (UUID(), 'Bayar Nutripangan', 1, 23);
