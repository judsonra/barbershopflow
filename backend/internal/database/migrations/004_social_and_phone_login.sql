ALTER TABLE users
    ADD COLUMN customer_id uuid REFERENCES customers(id),
    ADD COLUMN phone text,
    ADD COLUMN google_id text,
    ADD COLUMN facebook_id text,
    ADD COLUMN failed_login_attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN locked_at timestamptz;

ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- Cliente precisa apontar para um registro em customers; e-mail ou celular
-- identifica a conta; e ao menos uma forma de autenticar (senha ou provedor
-- social) precisa existir.
ALTER TABLE users ADD CONSTRAINT users_client_customer_id_chk CHECK (role <> 'client' OR customer_id IS NOT NULL);
ALTER TABLE users ADD CONSTRAINT users_identifier_chk CHECK (email IS NOT NULL OR phone IS NOT NULL);
ALTER TABLE users ADD CONSTRAINT users_credential_chk CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL OR facebook_id IS NOT NULL);

CREATE UNIQUE INDEX users_phone_idx ON users (phone) WHERE phone IS NOT NULL;
CREATE UNIQUE INDEX users_google_id_idx ON users (google_id) WHERE google_id IS NOT NULL;
CREATE UNIQUE INDEX users_facebook_id_idx ON users (facebook_id) WHERE facebook_id IS NOT NULL;
CREATE UNIQUE INDEX users_customer_id_idx ON users (customer_id) WHERE customer_id IS NOT NULL;
