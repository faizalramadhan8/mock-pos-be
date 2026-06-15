-- Tiered pricing untuk member: admin set "beli ≥ N satuan dapat harga X"
-- khusus member. Bisa target "semua member" atau "member tertentu" via
-- whitelist. Walk-in customer (non-member) selalu pakai selling_price normal.
--
-- min_qty disimpan dalam SATUAN, bukan dus — kalau cart pakai unit_type=box,
-- BE/FE harus convert (qty × qty_per_box) sebelum compare ke min_qty.

CREATE TABLE product_price_tiers (
  id          VARCHAR(36) PRIMARY KEY,
  product_id  VARCHAR(36) NOT NULL,
  min_qty     INT NOT NULL,
  price       DECIMAL(15,2) NOT NULL,
  target_type VARCHAR(20) NOT NULL,  -- 'all_members' | 'member_specific'
  note        VARCHAR(200) NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_ppt_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
  INDEX idx_ppt_product (product_id, min_qty)
);

-- Whitelist member untuk tier dengan target_type='member_specific'.
-- Kalau target_type='all_members', tabel ini kosong untuk tier itu.
CREATE TABLE product_price_tier_members (
  tier_id    VARCHAR(36) NOT NULL,
  member_id  VARCHAR(36) NOT NULL,
  PRIMARY KEY (tier_id, member_id),
  CONSTRAINT fk_pptm_tier   FOREIGN KEY (tier_id)   REFERENCES product_price_tiers(id) ON DELETE CASCADE,
  CONSTRAINT fk_pptm_member FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE CASCADE,
  INDEX idx_pptm_member (member_id)
);
