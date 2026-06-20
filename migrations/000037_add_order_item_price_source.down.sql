DROP INDEX idx_order_items_tier_id ON order_items;
DROP INDEX idx_order_items_price_source ON order_items;
ALTER TABLE order_items
  DROP FOREIGN KEY fk_order_items_tier_id,
  DROP COLUMN tier_id,
  DROP COLUMN price_source;
