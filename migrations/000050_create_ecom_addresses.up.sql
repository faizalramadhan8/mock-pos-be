-- Ecom customer address book (Bu Santi 24 Jul 2026).
-- Migration 000048 sempet buat `member_addresses` (FK ke members), tapi
-- ternyata customer register ecom = create USER row (bukan member). Table
-- itu belum dipakai code manapun — biarkan sebagai future member-side
-- address (kalau Bu Santi butuh input alamat member offline di POS).
--
-- ecom_addresses = dedicated untuk customer ecom, FK ke users.

CREATE TABLE ecom_addresses (
  id              VARCHAR(36) NOT NULL,
  user_id         VARCHAR(36) NOT NULL,

  label           VARCHAR(50) NOT NULL,       -- "Rumah", "Toko", "Gudang"
  recipient_name  VARCHAR(200) NOT NULL,
  recipient_phone VARCHAR(20) NOT NULL,

  -- Address cascade — Biteship-compatible naming.
  province        VARCHAR(100) NOT NULL,
  city            VARCHAR(100) NOT NULL,
  district        VARCHAR(100) NOT NULL,      -- Kecamatan
  subdistrict     VARCHAR(100) NOT NULL,      -- Kelurahan / Desa
  zipcode         VARCHAR(10)  NOT NULL,
  street_address  TEXT NOT NULL,

  -- Optional geo untuk future instant courier accuracy.
  latitude        DECIMAL(10,7) NULL,
  longitude       DECIMAL(10,7) NULL,
  notes           TEXT NULL,

  -- 1 default per user untuk fast-checkout.
  is_default      BOOLEAN NOT NULL DEFAULT FALSE,

  created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at      TIMESTAMP NULL,

  PRIMARY KEY (id),
  CONSTRAINT fk_ecom_addresses_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  KEY idx_ecom_addresses_user (user_id),
  KEY idx_ecom_addresses_default (user_id, is_default),
  KEY idx_ecom_addresses_deleted (deleted_at)
);
