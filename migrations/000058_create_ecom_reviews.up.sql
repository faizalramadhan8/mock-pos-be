-- Customer reviews & ratings untuk produk ecom.
-- Sprint 5b, Bu Santi 28 Jul 2026.
--
-- Rules:
--   - Cuma customer yang punya `completed` order dengan produk ini yang bisa review.
--     (BE enforce di POST — cegah review-spam / competitor sabotage.)
--   - 1 user × 1 produk = 1 review (UNIQUE). Boleh edit review lama.
--   - Rating 1-5.
--   - Comment optional (nullable).
--   - Admin bisa hide review (is_hidden) tanpa hapus — kalau kata2nya toxic.
CREATE TABLE IF NOT EXISTS ecom_reviews (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    product_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    order_id VARCHAR(36) NULL, -- reference ke order pertama yang trigger review
    rating TINYINT NOT NULL, -- 1..5
    comment TEXT NULL,
    is_hidden TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_review_user_product (user_id, product_id),
    KEY idx_review_product (product_id, is_hidden, created_at),
    CONSTRAINT fk_review_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
    CONSTRAINT fk_review_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
