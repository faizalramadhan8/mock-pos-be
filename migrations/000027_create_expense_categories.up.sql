-- Expense Categories — chart of accounts untuk pengeluaran operasional.
-- Bu Santi: "biar dalam sebulan ketahuan apa aja pengeluaran selain faktur
-- supplier (plastik, gaji, listrik, dll)". Kategori awal mengacu standar
-- SAK EMKM (Beban Pegawai, Beban Operasional, dll) tapi pakai label yang
-- mudah dipahami orang non-akuntan.
--
-- Schema decisions:
--   - is_system flag — kategori bawaan tidak bisa dihapus (cuma di-deactivate).
--   - sort_order — supaya urutan dropdown konsisten + bisa di-tweak nanti.
--   - is_active — soft-disable tanpa kehilangan history pengeluaran lama.

CREATE TABLE IF NOT EXISTS expense_categories (
    id          varchar(36)  NOT NULL PRIMARY KEY,
    name        varchar(100) NOT NULL,
    is_system   tinyint(1)   NOT NULL DEFAULT 0,
    is_active   tinyint(1)   NOT NULL DEFAULT 1,
    sort_order  int          NOT NULL DEFAULT 0,
    created_at  timestamp    NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  timestamp    NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_ec_active (is_active, sort_order)
);

INSERT INTO expense_categories (id, name, is_system, sort_order) VALUES
  (UUID(), 'Gaji & Lemburan Pegawai', 1, 10),
  (UUID(), 'Plastik & Kemasan',     1, 20),
  (UUID(), 'Listrik',               1, 30),
  (UUID(), 'Air',                   1, 40),
  (UUID(), 'Internet & Telepon',    1, 50),
  (UUID(), 'Sewa Tempat',           1, 60),
  (UUID(), 'Transport & Bensin',    1, 70),
  (UUID(), 'Perbaikan & Pemeliharaan', 1, 80),
  (UUID(), 'Pemasaran & Promosi',   1, 90),
  (UUID(), 'Administrasi',          1, 100),
  (UUID(), 'Pajak & Retribusi',     1, 110),
  (UUID(), 'Lain-lain',             1, 999);
