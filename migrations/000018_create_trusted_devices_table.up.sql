CREATE TABLE trusted_devices (
    id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    fingerprint VARCHAR(100) NOT NULL,
    status ENUM('pending','approved','rejected') NOT NULL DEFAULT 'pending',
    approval_code VARCHAR(64) NULL,
    code_expires_at TIMESTAMP NULL,
    name VARCHAR(100) NULL,
    user_agent VARCHAR(255) NULL,
    approved_at TIMESTAMP NULL,
    last_used_at TIMESTAMP NULL,
    last_notified_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uk_trusted_devices_user_fp (user_id, fingerprint),
    KEY idx_trusted_devices_code (approval_code),
    CONSTRAINT fk_trusted_devices_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
