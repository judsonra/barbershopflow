CREATE TYPE user_role AS ENUM ('manager', 'professional');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role user_role NOT NULL,
    professional_id uuid REFERENCES professionals(id),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (role = 'professional' OR professional_id IS NULL)
);

CREATE UNIQUE INDEX users_email_idx ON users (lower(email));

-- Usuario gestor inicial. Senha "change-me" - trocar imediatamente apos o primeiro login.
INSERT INTO users(name, email, password_hash, role) VALUES
    ('Administrador', 'admin@barberflow.local', '$2a$10$w0p5Uu8BAAEBs2pM44eK7OuxVsW4gmCgAk49fSCj/M56PeJacasIa', 'manager');
