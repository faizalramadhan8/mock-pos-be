DROP INDEX idx_order_items_redeemable_id ON order_items;
ALTER TABLE order_items
  DROP FOREIGN KEY fk_order_items_redeemable_id,
  DROP COLUMN redeemable_item_id;
