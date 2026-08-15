-- Separa "identidade" (quem e a pessoa: email/celular/senha/social) de
-- "vinculo" (em qual barbearia ela atua e com que papel). Ate aqui, users
-- misturava as duas coisas numa linha so, o que impedia a mesma pessoa
-- (ex: um cliente) de ter conta em mais de uma barbearia - email/celular
-- eram unicos globalmente E carregavam tenant_id/role/customer_id/
-- professional_id na mesma linha.
--
-- identities.id e memberships.id reaproveitam o mesmo uuid que a linha
-- tinha em users (backfill 1:1, sem ambiguidade nesse momento - so
-- vinculos futuros criam um segundo membership pra uma identidade ja
-- existente). Como o JWT (Claims.UserID) passa a significar "membership
-- id" em vez de "user id", tokens/refresh tokens ja emitidos continuam
-- resolvendo certinho depois desta migracao - zero deslogamento forcado.

CREATE TABLE identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    email text,
    phone text,
    password_hash text,
    google_id text,
    facebook_id text,
    failed_login_attempts integer NOT NULL DEFAULT 0,
    locked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (email IS NOT NULL OR phone IS NOT NULL),
    CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL OR facebook_id IS NOT NULL)
);

INSERT INTO identities (id, name, email, phone, password_hash, google_id, facebook_id, failed_login_attempts, locked_at, created_at)
SELECT id, name, email, phone, password_hash, google_id, facebook_id, failed_login_attempts, locked_at, created_at FROM users;

CREATE UNIQUE INDEX identities_email_idx ON identities (lower(email)) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX identities_phone_idx ON identities (phone) WHERE phone IS NOT NULL;
CREATE UNIQUE INDEX identities_google_id_idx ON identities (google_id) WHERE google_id IS NOT NULL;
CREATE UNIQUE INDEX identities_facebook_id_idx ON identities (facebook_id) WHERE facebook_id IS NOT NULL;

CREATE TABLE memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id uuid NOT NULL REFERENCES identities(id),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    role user_role NOT NULL,
    professional_id uuid REFERENCES professionals(id),
    customer_id uuid REFERENCES customers(id),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (role = 'professional' OR professional_id IS NULL),
    CHECK (role <> 'client' OR customer_id IS NOT NULL)
);

INSERT INTO memberships (id, identity_id, tenant_id, role, professional_id, customer_id, active, created_at)
SELECT id, id, tenant_id, role, professional_id, customer_id, active, created_at FROM users;

CREATE UNIQUE INDEX memberships_customer_id_idx ON memberships (customer_id) WHERE customer_id IS NOT NULL;
CREATE UNIQUE INDEX memberships_professional_id_idx ON memberships (professional_id) WHERE professional_id IS NOT NULL;
CREATE INDEX memberships_tenant_id_idx ON memberships (tenant_id);
CREATE INDEX memberships_identity_id_idx ON memberships (identity_id);

DROP TABLE users;
