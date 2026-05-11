-- Purchase Invoice (Faktur Pembelian) — multi-line invoice dari supplier.
-- Ganti workflow lama "Catat Faktur Barang Masuk" (1 produk per submit) yang
-- sebenarnya cuma stock-in per-produk. Sekarang real invoice: 1 header
-- supplier + N line items dalam satu transaksi atomic.
--
-- Schema decisions (Bu Santi confirmed):
--   - invoice_number EDITABLE + OPSIONAL bebas (boleh kosong, boleh duplicate).
--     Format supplier varies (MRA-2026-105001, dll) — jangan dipaksa unik.
--   - PPN dipisah subtotal + ppn_amount + total_amount supaya audit detail.
--     Default 11% auto-calc di FE; user bisa override (UMKM supplier no-PPN).
--   - reminder_sent_at — track H-0 WA reminder. Cron skip kalau already sent.
--   - payment_terms reuses existing string convention: COD / NET7 / NET14 / NET30.

CREATE TABLE IF NOT EXISTS purchase_invoices (
    id                varchar(36)    NOT NULL PRIMARY KEY,
    invoice_number    varchar(50)    NULL,
    supplier_id       varchar(36)    NOT NULL,
    invoice_date      timestamp      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    due_date          timestamp      NULL,
    payment_terms     varchar(20)    NOT NULL DEFAULT 'COD',
    payment_status    varchar(20)    NOT NULL DEFAULT 'unpaid',
    paid_at           timestamp      NULL,
    subtotal_amount   decimal(15,2)  NOT NULL DEFAULT 0,
    ppn_amount        decimal(15,2)  NOT NULL DEFAULT 0,
    total_amount      decimal(15,2)  NOT NULL DEFAULT 0,
    reminder_sent_at  timestamp      NULL,
    note              text           NULL,
    created_by        varchar(36)    NOT NULL,
    created_at        timestamp      NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        timestamp      NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at        timestamp      NULL,
    KEY idx_pi_supplier      (supplier_id),
    KEY idx_pi_status        (payment_status),
    KEY idx_pi_due           (payment_status, due_date),
    KEY idx_pi_deleted       (deleted_at),
    CONSTRAINT fk_pi_supplier
        FOREIGN KEY (supplier_id) REFERENCES suppliers(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS purchase_invoice_items (
    id                    varchar(36)   NOT NULL PRIMARY KEY,
    purchase_invoice_id   varchar(36)   NOT NULL,
    product_id            varchar(36)   NOT NULL,
    quantity              int           NOT NULL DEFAULT 0,
    unit_type             varchar(20)   NOT NULL DEFAULT 'individual',
    unit_price            decimal(15,2) NOT NULL DEFAULT 0,
    expiry_date           date          NULL,
    batch_id              varchar(36)   NULL,
    movement_id           varchar(36)   NULL,
    note                  text          NULL,
    created_at            timestamp     NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_pii_invoice  (purchase_invoice_id),
    KEY idx_pii_product  (product_id),
    CONSTRAINT fk_pii_invoice
        FOREIGN KEY (purchase_invoice_id) REFERENCES purchase_invoices(id) ON DELETE CASCADE,
    CONSTRAINT fk_pii_product
        FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT
);
