-- Capital Injections — setoran modal owner di luar penjualan.
-- Per request Bu Santi 21 Jun 2026: "ada penambahan modal dari owner dll".
--
-- Use case: Bu Santi inject uang pribadi ke kas toko untuk modal operasional
-- (mis. tambah stok besar, bayar supplier overdue, dll). Bukan pengeluaran,
-- bukan revenue — masuk sebagai +ModalTambahan di Arus Kas.
--
-- Pengaruh ke Arus Kas:
--   Saldo Akhir = Saldo Awal + Omzet + ModalTambahan − Pengeluaran
CREATE TABLE capital_injections (
  id          VARCHAR(36) PRIMARY KEY,
  amount      DECIMAL(15,2) NOT NULL,
  source      VARCHAR(100) NULL,        -- 'Owner', 'Pinjaman Bank', 'Investor', dll
  note        VARCHAR(500) NULL,
  injected_at DATETIME NOT NULL,        -- tanggal setoran (admin set)
  created_by  VARCHAR(36) NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at  TIMESTAMP NULL,
  INDEX idx_injected_at (injected_at),
  INDEX idx_deleted_at (deleted_at)
);
