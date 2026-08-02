-- Jornada de trabalho: janela semanal recorrente por profissional. Sem
-- nenhuma linha para o profissional = sem restricao (retrocompatibilidade
-- com profissionais criados antes dessa feature). Com ao menos uma linha,
-- um dia da semana sem linha correspondente vira dia de folga fixo.
CREATE TABLE professional_schedules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    professional_id uuid NOT NULL REFERENCES professionals(id),
    weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_minute integer NOT NULL CHECK (start_minute >= 0 AND start_minute < 1440),
    end_minute integer NOT NULL CHECK (end_minute > start_minute AND end_minute <= 1440),
    UNIQUE (professional_id, weekday)
);

CREATE INDEX professional_schedules_professional_id_idx ON professional_schedules(professional_id);

-- Bloqueios pontuais: ausencia, folga, viagem etc. Qualquer sobreposicao com
-- [starts_at, ends_at) barra a criacao de agendamento nesse profissional.
CREATE TABLE professional_time_off (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    professional_id uuid NOT NULL REFERENCES professionals(id),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL CHECK (ends_at > starts_at),
    reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX professional_time_off_professional_id_idx ON professional_time_off(professional_id, starts_at);
