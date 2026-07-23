-- E-commerce auth extension (Bu Santi 20 Jul 2026).
--
-- Design decisions per discussion:
--
-- 1. USERS TABLE: tambah 2 role baru `ecom_admin` + `ecom_superadmin`. Cegah
--    kasir/staff toko punya access ke ecom panel, dan sebaliknya. superadmin
--    (existing) bisa akses semua sistem (POS + ecom admin) — Bu Santi 1 akun
--    untuk kedua sistem.
--
--    Role hierarchy setelah migration:
--      superadmin        — akses semua (POS + ecom admin)
--      admin             — POS admin (existing)
--      staff             — POS staff toko (existing)
--      cashier           — POS kasir (existing)
--      user              — legacy (existing)
--      ecom_admin        — hanya ecom admin panel (baru)
--      ecom_superadmin   — hanya ecom admin panel + user mgmt di sana (baru)
--
-- 2. MEMBERS TABLE: extend jadi customer table sekaligus. Kolom baru:
--      email             — unique kalau non-NULL. NULL untuk member offline
--                          yang cuma Bu Santi input via POS (no ecom account).
--      password_hash     — bcrypt hash. NULL untuk member offline-only.
--      email_verified_at — verify status (OTP via WA di v1, email confirm v2).
--      ecom_registered_at — flag: kapan member ini register via ecom. NULL =
--                          member offline created via POS (existing).
--
--    Auto-merge behavior: customer register di ecom pakai phone. Kalau phone
--    match existing member (yang Bu Santi input via POS), auto-link — customer
--    dapat riwayat point + benefit member yang sudah accumulated. Cegah data
--    duplikat + Bu Santi tidak perlu manual merge.
--
-- 3. MEMBER_ADDRESSES TABLE (BARU): address book per member untuk checkout.
--    Design mirror pattern Tokopedia:
--      - Max ~5 address per member (soft limit di FE, no DB constraint)
--      - 1 harus `is_default = TRUE` untuk fast-checkout
--      - Cascade Provinsi → Kota → Kecamatan → Kelurahan (Biteship-compatible)

-- ─── USERS role expand ───────────────────────────────────────────────
ALTER TABLE users
  MODIFY COLUMN role ENUM(
    'user','admin','superadmin','cashier','staff','ecom_admin','ecom_superadmin'
  ) NOT NULL DEFAULT 'user';

-- ─── MEMBERS extend jadi customer ────────────────────────────────────
ALTER TABLE members
  ADD COLUMN email VARCHAR(200) NULL AFTER phone,
  ADD COLUMN password_hash VARCHAR(255) NULL AFTER email,
  ADD COLUMN email_verified_at DATETIME NULL AFTER password_hash,
  ADD COLUMN ecom_registered_at DATETIME NULL AFTER email_verified_at;

-- Unique email hanya untuk yang non-NULL (multiple member offline tetap boleh
-- NULL email). MySQL 8.0 unique-null-friendly by default.
CREATE UNIQUE INDEX uk_members_email ON members(email);

-- ─── MEMBER_ADDRESSES: address book untuk checkout ───────────────────
CREATE TABLE member_addresses (
  id             VARCHAR(36) NOT NULL,
  member_id      VARCHAR(36) NOT NULL,

  -- Label bebas ("Rumah", "Toko", "Gudang") untuk fast recall di checkout.
  label          VARCHAR(50) NOT NULL,

  -- Recipient — bisa beda dari member (mis. member kirim ke asisten).
  recipient_name  VARCHAR(200) NOT NULL,
  recipient_phone VARCHAR(20) NOT NULL,

  -- Address cascade — Biteship-compatible naming (avoid rename waktu integrate).
  province       VARCHAR(100) NOT NULL,
  city           VARCHAR(100) NOT NULL,
  district       VARCHAR(100) NOT NULL,     -- Kecamatan
  subdistrict    VARCHAR(100) NOT NULL,     -- Kelurahan / Desa
  zipcode        VARCHAR(10)  NOT NULL,

  -- Full street address free-text (jl, nomor, blok, patokan).
  street_address TEXT NOT NULL,

  -- Lat/lng optional — untuk future map picker / instant courier accuracy.
  latitude       DECIMAL(10,7) NULL,
  longitude      DECIMAL(10,7) NULL,

  -- Notes untuk kurir (mis. "warna pagar hijau, ketuk pelan").
  notes          TEXT NULL,

  -- Default flag — exactly 1 must be TRUE per member (enforced FE-side, atau
  -- via trigger di v2 kalau butuh). Fast-checkout pakai default.
  is_default     BOOLEAN NOT NULL DEFAULT FALSE,

  created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at     TIMESTAMP NULL,

  PRIMARY KEY (id),
  CONSTRAINT fk_member_addresses_member
    FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE CASCADE,
  KEY idx_member_addresses_member (member_id),
  KEY idx_member_addresses_default (member_id, is_default),
  KEY idx_member_addresses_deleted (deleted_at)
);
