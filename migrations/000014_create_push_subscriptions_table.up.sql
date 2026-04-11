CREATE TABLE push_subscriptions (
    id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    endpoint TEXT NOT NULL,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_push_subs_user_id (user_id),
    CONSTRAINT fk_push_subs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
