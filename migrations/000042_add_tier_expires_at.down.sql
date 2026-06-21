ALTER TABLE product_price_tier_history DROP COLUMN expires_at;
ALTER TABLE product_price_tiers
  DROP INDEX idx_expires_at,
  DROP COLUMN expires_at;
