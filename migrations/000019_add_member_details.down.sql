ALTER TABLE members
    DROP KEY uk_members_member_number,
    DROP KEY idx_members_name,
    DROP COLUMN member_number,
    DROP COLUMN address;
