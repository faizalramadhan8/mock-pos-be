DROP TABLE member_addresses;

DROP INDEX uk_members_email ON members;
ALTER TABLE members
  DROP COLUMN ecom_registered_at,
  DROP COLUMN email_verified_at,
  DROP COLUMN password_hash,
  DROP COLUMN email;

-- Rollback role ENUM ke definisi migration 000002 (sebelum ecom_admin).
ALTER TABLE users
  MODIFY COLUMN role ENUM(
    'user','admin','superadmin','cashier','staff'
  ) NOT NULL DEFAULT 'user';
