-- Copyright (c) 2024-2025 Darcy Buskermolen <darcy@dbitech.ca>
-- SPDX-License-Identifier: BSD-3-Clause
 
-- Token management table
CREATE TABLE api_tokens (
    token VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    valid_until TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    scopes TEXT[],
    description TEXT,
    last_used TIMESTAMP,
    created_by VARCHAR(255),
    is_active BOOLEAN DEFAULT true
);

-- Indexes
CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_valid_until ON api_tokens(valid_until);
CREATE INDEX idx_api_tokens_is_active ON api_tokens(is_active);

-- Example tokens
INSERT INTO api_tokens (
    token, 
    user_id, 
    valid_until, 
    scopes, 
    description
) VALUES (
    'dev-test-token-1',
    'test-user',
    NOW() + INTERVAL '30 days',
    ARRAY['transcribe'],
    'Development testing token'
);

-- Token usage tracking function
CREATE OR REPLACE FUNCTION update_token_last_used()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE api_tokens
    SET last_used = NOW()
    WHERE token = NEW.token;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Optional: Create a trigger to track token usage
-- CREATE TRIGGER track_token_usage
--     AFTER INSERT ON token_usage
--     FOR EACH ROW
--     EXECUTE FUNCTION update_token_last_used();
