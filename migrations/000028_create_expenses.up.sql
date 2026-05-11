-- Expenses — pengeluaran operasional toko di luar pembelian barang ke supplier
-- (yang sudah di-track via purchase_invoices). Mendukung Laporan Laba Rugi
-- bulanan: Untung = Omzet - HPP - Pengeluaran.
--
-- Schema decisions:
--   - employee_name optional text — diisi cuma kalau kategori "Beban Pegawai".
--     Bukan FK ke users supaya bisa catat pegawai gudang/non-system tanpa
--     bikin akun. Datalist FE auto-suggest dari users table.
--   - amount decimal(15,2) — sama dengan order/purchase_invoice totals.
--   - payment_method optional — cash / transfer / qris. Kalau kosong = cash.
--   - expense_date — tanggal terjadinya pengeluaran (boleh backdate). Bukan
--     created_at (waktu input ke sistem).
--   - category_id RESTRICT — cegah delete kategori yang masih ke-link.

CREATE TABLE IF NOT EXISTS expenses (
    id              varchar(36)    NOT NULL PRIMARY KEY,
    category_id     varchar(36)    NOT NULL,
    expense_date    date           NOT NULL,
    description     varchar(255)   NOT NULL,
    amount          decimal(15,2)  NOT NULL DEFAULT 0,
    employee_name   varchar(100)   NULL,
    payment_method  varchar(20)    NOT NULL DEFAULT 'cash',
    note            text           NULL,
    created_by      varchar(36)    NOT NULL,
    created_at      timestamp      NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      timestamp      NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at      timestamp      NULL,
    KEY idx_exp_date      (expense_date),
    KEY idx_exp_category  (category_id, expense_date),
    KEY idx_exp_deleted   (deleted_at),
    CONSTRAINT fk_exp_category
        FOREIGN KEY (category_id) REFERENCES expense_categories(id) ON DELETE RESTRICT
);
