ALTER TABLE professionals
    ADD COLUMN email text NOT NULL DEFAULT '',
    ADD COLUMN cpf text NOT NULL DEFAULT '';

-- Ambos opcionais, mas quando informados precisam ser unicos dentro da
-- barbearia (CPF por natureza identifica uma pessoa; e-mail e reservado
-- porque um futuro fluxo de login do profissional deve casar por e-mail,
-- como ja acontece hoje para clientes/staff em auth social - ver TODO.md).
CREATE UNIQUE INDEX professionals_email_idx ON professionals (tenant_id, lower(email)) WHERE email <> '';
CREATE UNIQUE INDEX professionals_cpf_idx ON professionals (tenant_id, cpf) WHERE cpf <> '';
