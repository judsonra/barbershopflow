-- Comissão por profissional e serviço: percentual (em centésimos de ponto
-- percentual, 0 a 10000 = 0% a 100%) ou valor fixo em centavos.
-- Renumerada de 021 para 022 porque a 021 passou a ser a identidade visual.
-- IF NOT EXISTS: bancos que já rodaram a versão 021 desta migration (como o
-- de dev) já têm a tabela, e a migração renomeada não pode falhar neles.
CREATE TABLE IF NOT EXISTS professional_commissions (
 tenant_id uuid NOT NULL REFERENCES tenants(id),
 professional_id uuid NOT NULL REFERENCES professionals(id),
 service_id uuid NOT NULL REFERENCES services(id),
 type text NOT NULL CHECK (type IN ('percentage', 'fixed')),
 value bigint NOT NULL CHECK (value >= 0 AND (type <> 'percentage' OR value <= 10000)),
 PRIMARY KEY (tenant_id, professional_id, service_id)
);
