ALTER TABLE users
  MODIFY COLUMN role ENUM('user','admin','superadmin','cashier','staff') NOT NULL DEFAULT 'user',
  ADD COLUMN nik VARCHAR(50) NULL AFTER role,
  ADD COLUMN date_of_birth DATE NULL AFTER nik,
  ADD COLUMN is_active TINYINT(1) NOT NULL DEFAULT 1 AFTER date_of_birth;
