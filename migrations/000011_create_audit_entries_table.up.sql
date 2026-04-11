CREATE TABLE audit_entries (
    id VARCHAR(36) NOT NULL,
    action VARCHAR(50) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    user_name VARCHAR(200) NOT NULL,
    details TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_audit_entries_action (action),
    KEY idx_audit_entries_user_id (user_id),
    KEY idx_audit_entries_created_at (created_at)
);
