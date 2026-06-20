-- Rename product_price_tiers.target_type 'all_members' → 'all_customers'.
-- Bu Santi 20 Jun 2026: tier "Semua Member" diganti jadi "Semua Customer" supaya
-- non-member walk-in juga bisa dapat harga grosir. Target 'member_specific' tetap.
UPDATE product_price_tiers SET target_type = 'all_customers' WHERE target_type = 'all_members';
