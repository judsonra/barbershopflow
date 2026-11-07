-- Identidade visual de cada barbearia (Config > Identidade visual): paleta de
-- cores escolhida entre as prontas e logo própria, usadas na página
-- http://dominio/<slug> e dentro do app. Barbearias existentes ficam com a
-- paleta "classico" (o visual original) e sem logo.
-- A imagem fica numa tabela à parte para que listar/consultar barbearias não
-- carregue os bytes; tenants.logo_updated_at só versiona a URL da logo.
ALTER TABLE tenants
  ADD COLUMN color_scheme text NOT NULL DEFAULT 'classico',
  ADD COLUMN logo_updated_at timestamptz;

CREATE TABLE tenant_logos (
  tenant_id uuid PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  content_type text NOT NULL,
  data bytea NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
