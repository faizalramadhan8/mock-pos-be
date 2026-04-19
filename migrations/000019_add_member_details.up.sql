ALTER TABLE members
    ADD COLUMN address TEXT NULL AFTER phone,
    ADD COLUMN member_number VARCHAR(50) NULL AFTER address,
    ADD UNIQUE KEY uk_members_member_number (member_number),
    ADD KEY idx_members_name (name);
