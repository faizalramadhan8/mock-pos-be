-- Cashbook opening balance per bulan. Owner input manual tiap awal bulan
-- (decide via AskUserQuestion 19 Jun 2026: "Selalu input manual per bulan"
-- daripada auto-carry, untuk flexibility koreksi).
--
-- Source of truth untuk laporan Arus Kas. Tanpa row → assume 0 (clean slate).
CREATE TABLE cashbook_opening_balances (
  id          VARCHAR(36) PRIMARY KEY,
  year        INT NOT NULL,
  month       INT NOT NULL,             -- 1-12
  balance     DECIMAL(15,2) NOT NULL,   -- saldo awal Rp
  note        TEXT NULL,
  created_by  VARCHAR(36) NOT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_year_month (year, month)
);
