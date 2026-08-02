CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (length(trim(name)) > 0),
    slug text NOT NULL CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'),
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX tenants_slug_idx ON tenants (slug);

-- Barbearia inicial: dona dos dados criados antes do multi-tenant existir.
INSERT INTO tenants(name, slug) VALUES ('BarberFlow Demo', 'demo');

ALTER TABLE users ADD COLUMN tenant_id uuid REFERENCES tenants(id);
ALTER TABLE services ADD COLUMN tenant_id uuid REFERENCES tenants(id);
ALTER TABLE professionals ADD COLUMN tenant_id uuid REFERENCES tenants(id);
ALTER TABLE customers ADD COLUMN tenant_id uuid REFERENCES tenants(id);
ALTER TABLE appointments ADD COLUMN tenant_id uuid REFERENCES tenants(id);

UPDATE users SET tenant_id = (SELECT id FROM tenants WHERE slug = 'demo');
UPDATE services SET tenant_id = (SELECT id FROM tenants WHERE slug = 'demo');
UPDATE professionals SET tenant_id = (SELECT id FROM tenants WHERE slug = 'demo');
UPDATE customers SET tenant_id = (SELECT id FROM tenants WHERE slug = 'demo');
UPDATE appointments SET tenant_id = (SELECT id FROM tenants WHERE slug = 'demo');

ALTER TABLE users ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE services ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE professionals ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE customers ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE appointments ALTER COLUMN tenant_id SET NOT NULL;

CREATE INDEX users_tenant_id_idx ON users(tenant_id);
CREATE INDEX services_tenant_id_idx ON services(tenant_id);
CREATE INDEX professionals_tenant_id_idx ON professionals(tenant_id);
CREATE INDEX customers_tenant_id_idx ON customers(tenant_id);
CREATE INDEX appointments_tenant_id_idx ON appointments(tenant_id);

-- E-mail/celular/ids sociais continuam globalmente unicos (login nao pede
-- para escolher a barbearia primeiro); o isolamento que importa e o dos
-- dados de negocio abaixo, agora tambem cobrindo a checagem de sobreposicao
-- de agenda por tenant (defesa em profundidade - professional_id ja e
-- unico por tenant).
ALTER TABLE appointments DROP CONSTRAINT appointments_no_overlap;
ALTER TABLE appointments ADD CONSTRAINT appointments_no_overlap
EXCLUDE USING gist (
    tenant_id WITH =,
    professional_id WITH =,
    tstzrange(starts_at, ends_at, '[)') WITH &&
) WHERE (status IN ('scheduled', 'confirmed'));
