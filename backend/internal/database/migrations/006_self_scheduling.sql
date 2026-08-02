-- Autoagendamento e confirmacao sao dois parametros independentes por
-- barbearia:
--   self_scheduling_enabled: cliente pode sugerir o proprio horario.
--   auto_confirm_appointments: quando o cliente agenda, o horario entra
--     como 'confirmed' direto (auto-confirmado) se true, ou como
--     'scheduled' (pendente, o profissional confirma manualmente) se false.
-- Agendamentos criados por staff (gestor/profissional) nao sao afetados por
-- nenhum dos dois - sempre comecam 'scheduled', como ja era.
ALTER TABLE tenants
    ADD COLUMN self_scheduling_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN auto_confirm_appointments boolean NOT NULL DEFAULT false;
