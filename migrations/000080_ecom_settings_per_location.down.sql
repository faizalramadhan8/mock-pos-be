DELETE FROM ecom_settings WHERE id = 'store';
DROP INDEX idx_ecom_settings_location ON ecom_settings;
ALTER TABLE ecom_settings DROP COLUMN location;
