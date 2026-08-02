CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 5 AND 480),
    price_cents bigint NOT NULL CHECK (price_cents >= 0),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE professionals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    phone text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    phone text NOT NULL DEFAULT '',
    email text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE appointment_status AS ENUM ('scheduled', 'confirmed', 'completed', 'cancelled');

CREATE TABLE appointments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id uuid NOT NULL REFERENCES customers(id),
    professional_id uuid NOT NULL REFERENCES professionals(id),
    service_id uuid NOT NULL REFERENCES services(id),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL CHECK (ends_at > starts_at),
    status appointment_status NOT NULL DEFAULT 'scheduled',
    notes text NOT NULL DEFAULT '',
    price_cents bigint NOT NULL CHECK (price_cents >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE appointments ADD CONSTRAINT appointments_no_overlap
EXCLUDE USING gist (
    professional_id WITH =,
    tstzrange(starts_at, ends_at, '[)') WITH &&
) WHERE (status IN ('scheduled', 'confirmed'));

CREATE INDEX appointments_starts_at_idx ON appointments(starts_at);

INSERT INTO services(name, duration_minutes, price_cents) VALUES
    ('Corte', 45, 5000),
    ('Barba', 30, 3500),
    ('Corte + Barba', 75, 7500);

INSERT INTO professionals(name, phone) VALUES ('Rafael', '(11) 99999-0000');
