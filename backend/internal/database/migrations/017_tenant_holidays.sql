-- Feriados: calendário de dias bloqueados compartilhado pela barbearia
-- inteira, diferente de professional_time_off (que bloqueia só um
-- profissional por vez). Nenhum agendamento é aceito na data do feriado,
-- para nenhum profissional.
CREATE TABLE tenant_holidays (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    holiday_date date NOT NULL,
    name text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, holiday_date)
);
CREATE INDEX tenant_holidays_tenant_id_idx ON tenant_holidays(tenant_id, holiday_date);
