-- Audit trail untuk perubahan product_price_tiers (create/update/delete).
-- Mirror pattern product_price_history (migration 000024).
--
-- Setiap CRUD tier insert satu row dengan snapshot lengkap (min_qty, price,
-- target_type, member whitelist). Pattern:
--   Create: insert row status='active', action='create', start_date=NOW
--   Update: close current active (status='inactive', end_date=NOW) + insert
--           new row status='active', action='update', start_date=NOW
--   Delete: close current active (status='inactive', end_date=NOW), action='delete'
--
-- member_ids di-snapshot sebagai JSON array untuk target_type='member_specific'
-- supaya audit tidak hilang kalau whitelist diubah.
CREATE TABLE product_price_tier_history (
  id           VARCHAR(36) PRIMARY KEY,
  tier_id      VARCHAR(36) NOT NULL,
  product_id   VARCHAR(36) NOT NULL,
  min_qty      INT NOT NULL,
  price        DECIMAL(15,2) NOT NULL,
  target_type  VARCHAR(20) NOT NULL,
  member_ids   JSON NULL,
  note         VARCHAR(200) NULL,
  status       VARCHAR(20) NOT NULL,
  action       VARCHAR(20) NOT NULL,
  start_date   DATETIME NOT NULL,
  end_date     DATETIME NULL,
  changed_by   VARCHAR(36) NULL,
  created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_tier_id (tier_id),
  INDEX idx_product_id (product_id),
  INDEX idx_status (status)
);

-- Backfill: untuk tier yang sudah ada, insert satu row status='active',
-- action='create', start_date=tier.created_at. Member whitelist di-snapshot
-- via subquery JSON_ARRAYAGG.
INSERT INTO product_price_tier_history (
  id, tier_id, product_id, min_qty, price, target_type, member_ids, note,
  status, action, start_date, changed_by, created_at
)
SELECT
  UUID(), t.id, t.product_id, t.min_qty, t.price, t.target_type,
  (SELECT JSON_ARRAYAGG(tm.member_id)
     FROM product_price_tier_members tm
     WHERE tm.tier_id = t.id),
  t.note,
  'active', 'create', t.created_at, NULL, t.created_at
FROM product_price_tiers t;
