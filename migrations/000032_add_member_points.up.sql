-- Member points: belanja kelipatan tepat Rp 100.000 dapat 1.000 poin per
-- kelipatan. Poin bisa dipakai kasir untuk "tebus" item per-row di cart
-- (poin >= harga item × qty). Reset 0 setiap 1 Januari via cron.

ALTER TABLE members ADD COLUMN points INT NOT NULL DEFAULT 0;

-- Flag per order_item: TRUE kalau item ini dibayar pakai poin (tebus),
-- sehingga harga item tidak masuk hitungan earn poin baru (cegah loop).
ALTER TABLE order_items ADD COLUMN redeemed_with_points BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE member_point_movements (
  id VARCHAR(36) PRIMARY KEY,
  member_id VARCHAR(36) NOT NULL,
  order_id VARCHAR(36) NULL,
  type VARCHAR(20) NOT NULL,
  points INT NOT NULL,
  balance_after INT NOT NULL,
  note TEXT NULL,
  created_by VARCHAR(36) NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_mpm_member  FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE CASCADE,
  CONSTRAINT fk_mpm_order   FOREIGN KEY (order_id)  REFERENCES orders(id)  ON DELETE SET NULL,
  INDEX idx_mpm_member_created (member_id, created_at)
);
