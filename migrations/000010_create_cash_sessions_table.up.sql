CREATE TABLE cash_sessions (
    id VARCHAR(36) NOT NULL,
    date DATE NOT NULL,
    opening_cash DECIMAL(15,2) NOT NULL DEFAULT 0,
    opened_by VARCHAR(200) NOT NULL,
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expected_cash DECIMAL(15,2) NULL DEFAULT 0,
    actual_cash DECIMAL(15,2) NULL DEFAULT 0,
    difference DECIMAL(15,2) NULL DEFAULT 0,
    notes TEXT NULL,
    closed_by VARCHAR(200) NULL,
    closed_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_cash_sessions_date (date)
);
