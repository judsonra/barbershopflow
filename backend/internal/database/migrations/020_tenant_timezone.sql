-- Fuso horário de cada barbearia, usado para interpretar jornada, feriados e
-- períodos de relatório (cadastrados em hora local). Barbearias existentes
-- ficam no horário de Brasília e podem trocar em Config > Barbearia.
-- Horários já gravados em timestamptz não mudam: são instantes absolutos.
ALTER TABLE tenants ADD COLUMN timezone text NOT NULL DEFAULT 'America/Sao_Paulo';
