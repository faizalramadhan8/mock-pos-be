-- Per request Bu Santi 24 Jun 2026 — owner mau record Prive (penarikan kas
-- pemilik untuk keperluan pribadi). Kebalikan dari Tambahan Modal (setoran).
--
-- Same table capital_injections, beda type. Type values:
--   'injection' = setoran owner ke kas toko (+saldo)
--   'drawing'   = prive: penarikan owner dari kas toko (-saldo)
--
-- Default 'injection' untuk preserve existing data (semua row pre-migration
-- = setoran modal, bukan prive).
ALTER TABLE capital_injections
  ADD COLUMN type VARCHAR(20) NOT NULL DEFAULT 'injection';

CREATE INDEX idx_capital_type ON capital_injections(type);
