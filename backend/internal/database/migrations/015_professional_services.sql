-- Especialidades por profissional: define quais serviços cada profissional
-- pode executar, com preço/duração opcionais que sobrescrevem os valores
-- padrão do serviço. Um profissional sem nenhuma linha aqui mantém o
-- comportamento anterior (retrocompatível): qualquer serviço ativo pode ser
-- agendado para ele, usando preço/duração do próprio serviço. A partir da
-- primeira linha cadastrada, só os serviços listados podem ser agendados
-- para aquele profissional.
CREATE TABLE professional_services (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    professional_id uuid NOT NULL REFERENCES professionals(id),
    service_id uuid NOT NULL REFERENCES services(id),
    price_cents_override bigint CHECK (price_cents_override IS NULL OR price_cents_override >= 0),
    duration_minutes_override integer CHECK (duration_minutes_override IS NULL OR duration_minutes_override > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (professional_id, service_id)
);
CREATE INDEX professional_services_professional_id_idx ON professional_services(professional_id);
CREATE INDEX professional_services_service_id_idx ON professional_services(service_id);
