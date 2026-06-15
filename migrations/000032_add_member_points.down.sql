DROP TABLE IF EXISTS member_point_movements;
ALTER TABLE order_items DROP COLUMN redeemed_with_points;
ALTER TABLE members DROP COLUMN points;
