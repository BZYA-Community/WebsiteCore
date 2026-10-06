ALTER TABLE p_user ADD COLUMN email VARCHAR(254) NOT NULL DEFAULT '';
CREATE UNIQUE INDEX idx_user_active_email ON p_user(lower(email)) WHERE email <> '' AND is_del = 0;
CREATE UNIQUE INDEX idx_user_active_phone ON p_user(phone) WHERE phone <> '' AND is_del = 0;

CREATE TABLE p_contact_verification (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES p_user(id),
    mode VARCHAR(5) NOT NULL CHECK (mode IN ('email', 'phone')),
    address VARCHAR(254) NOT NULL,
    ip_hash VARCHAR(64) NOT NULL,
    code_hash VARCHAR(64) NOT NULL,
    created_on BIGINT NOT NULL,
    expires_on BIGINT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    delivered BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX idx_contact_verification_user ON p_contact_verification(user_id, id DESC);
CREATE INDEX idx_contact_verification_address ON p_contact_verification(address, created_on);
CREATE INDEX idx_contact_verification_ip ON p_contact_verification(ip_hash, created_on);
