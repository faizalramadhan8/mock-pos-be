DROP INDEX idx_product_price_tiers_starts ON product_price_tiers;
ALTER TABLE product_price_tiers DROP COLUMN starts_at;
ALTER TABLE product_price_tier_history DROP COLUMN starts_at;
