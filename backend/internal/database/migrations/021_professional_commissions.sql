CREATE TABLE professional_commissions (
 tenant_id uuid NOT NULL REFERENCES tenants(id),
 professional_id uuid NOT NULL REFERENCES professionals(id),
 service_id uuid NOT NULL REFERENCES services(id),
 type text NOT NULL CHECK (type IN ('percentage', 'fixed')),
 value bigint NOT NULL CHECK (value >= 0 AND (type <> 'percentage' OR value <= 10000)),
 PRIMARY KEY (tenant_id, professional_id, service_id)
);
