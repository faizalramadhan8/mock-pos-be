-- Order ecom-specific fields (Bu Santi 24 Jul 2026).
-- Support checkout flow: alamat snapshot, kurir, ongkir, payment gateway
-- reference, dan status pipeline khusus ecom.

ALTER TABLE orders
  -- User ecom yang order (link ke users.id). NULL untuk order POS (existing).
  ADD COLUMN ecom_user_id VARCHAR(36) NULL AFTER order_source,

  -- Alamat snapshot (JSON) — cegah kalau customer edit/hapus alamat setelah
  -- checkout, order tetap punya info kirim.
  ADD COLUMN shipping_address_snapshot JSON NULL,

  -- Kurir + ongkir yang customer pilih di checkout.
  ADD COLUMN shipping_courier VARCHAR(50) NULL,       -- "JNE", "JNT", "GoSend"
  ADD COLUMN shipping_service VARCHAR(50) NULL,       -- "Reguler", "YES", "Instant"
  ADD COLUMN shipping_cost DECIMAL(15,2) NULL DEFAULT 0,
  ADD COLUMN shipping_etd VARCHAR(50) NULL,           -- "2-3 hari"

  -- Airway bill (resi) — di-input Bu Santi setelah kirim, atau auto dari
  -- Biteship kalau integrasi order-nya juga (Fase 4+).
  ADD COLUMN shipping_awb VARCHAR(100) NULL,

  -- Ecom-specific status pipeline (beda dari POS status='completed').
  -- pending_payment → paid → processing → shipped → delivered → completed
  -- Atau cancelled kapan saja.
  ADD COLUMN ecom_status VARCHAR(30) NULL,

  -- Midtrans Snap integration.
  ADD COLUMN payment_snap_token VARCHAR(100) NULL,
  ADD COLUMN payment_reference VARCHAR(100) NULL,     -- transaction_id from Midtrans
  ADD COLUMN payment_paid_at DATETIME NULL,
  ADD COLUMN payment_expired_at DATETIME NULL;        -- auto-expire kalau tidak bayar

CREATE INDEX idx_orders_ecom_user ON orders(ecom_user_id);
CREATE INDEX idx_orders_ecom_status ON orders(ecom_status);
CREATE INDEX idx_orders_payment_ref ON orders(payment_reference);
