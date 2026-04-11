CREATE TABLE categories (
    id VARCHAR(36) NOT NULL,
    name VARCHAR(100) NOT NULL,
    name_id VARCHAR(100) NOT NULL,
    icon VARCHAR(50) NULL,
    color VARCHAR(20) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    PRIMARY KEY (id),
    KEY idx_categories_deleted_at (deleted_at)
);

INSERT INTO categories (id, name, name_id, icon, color) VALUES
('c1', 'Flour & Starch', 'Tepung & Pati', 'flour', '#C4884A'),
('c2', 'Sugar', 'Gula', 'sugar', '#D4627A'),
('c3', 'Dairy & Eggs', 'Susu & Telur', 'dairy', '#5B8DEF'),
('c4', 'Chocolate', 'Cokelat', 'choco', '#7D5A44'),
('c5', 'Leavening', 'Pengembang', 'leaven', '#8B6FC0'),
('c6', 'Nuts & Fruits', 'Kacang & Buah', 'nuts', '#6F9A4D'),
('c7', 'Fats & Oils', 'Lemak & Minyak', 'fats', '#E89B48'),
('c8', 'Flavors', 'Perasa & Ekstrak', 'flavor', '#2BA5B5');
