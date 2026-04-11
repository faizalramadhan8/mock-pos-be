ALTER TABLE settings
    ADD COLUMN label_width INT NOT NULL DEFAULT 40 AFTER ppn_rate,
    ADD COLUMN label_height INT NOT NULL DEFAULT 30 AFTER label_width;
