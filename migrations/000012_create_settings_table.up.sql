CREATE TABLE settings (
    id VARCHAR(36) NOT NULL,
    store_name VARCHAR(200) NOT NULL DEFAULT 'Bakeshop',
    store_address TEXT NULL,
    store_phone VARCHAR(20) NULL,
    ppn_rate DECIMAL(5,2) NOT NULL DEFAULT 11,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
);

CREATE TABLE bank_accounts (
    id VARCHAR(36) NOT NULL,
    settings_id VARCHAR(36) NOT NULL,
    bank_name VARCHAR(100) NOT NULL,
    account_number VARCHAR(50) NOT NULL,
    account_holder VARCHAR(200) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_bank_accounts_settings_id (settings_id),
    CONSTRAINT fk_bank_accounts_settings FOREIGN KEY (settings_id) REFERENCES settings(id) ON DELETE CASCADE
);

INSERT INTO settings (id, store_name, ppn_rate) VALUES ('default', 'Bakeshop', 11);
