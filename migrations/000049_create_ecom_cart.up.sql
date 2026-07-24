-- E-commerce cart persistence (Bu Santi 24 Jul 2026).
-- MySQL table (bukan Redis) — persistent cross-session, mudah inspect, no TTL
-- expire yang bisa bikin customer kehilangan cart.
--
-- user_id FK ke users(id): customer register ecom create user row (bukan
-- member row). Members table = customer POS-created oleh Bu Santi di counter.
-- Kalau nanti mau cross-link customer online ↔ member offline (untuk loyalty
-- points), tambah `members.user_id` di migration terpisah.
--
-- UNIQUE (user_id, product_id) — 1 row per produk. Add product yang sudah
-- ada di cart = increment quantity (bukan insert row duplikat).

CREATE TABLE ecom_cart_items (
  id          VARCHAR(36) NOT NULL,
  user_id     VARCHAR(36) NOT NULL,
  product_id  VARCHAR(36) NOT NULL,
  quantity    INT NOT NULL DEFAULT 1,
  created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

  PRIMARY KEY (id),
  UNIQUE KEY uk_ecom_cart_user_product (user_id, product_id),
  KEY idx_ecom_cart_user (user_id),
  CONSTRAINT fk_ecom_cart_user    FOREIGN KEY (user_id)    REFERENCES users(id)    ON DELETE CASCADE,
  CONSTRAINT fk_ecom_cart_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);
