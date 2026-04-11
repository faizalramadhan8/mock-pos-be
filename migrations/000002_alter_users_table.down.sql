ALTER TABLE users
  DROP COLUMN is_active,
  DROP COLUMN date_of_birth,
  DROP COLUMN nik,
  MODIFY COLUMN role ENUM('user','admin','superadmin') NOT NULL DEFAULT 'user';
