-- Adiciona o estado terminal no_show ao enum de status de agendamento,
-- para o profissional/gestor marcar quando o cliente não aparece
-- (distinto de um cancelamento avisado com antecedência).
ALTER TYPE appointment_status ADD VALUE 'no_show';
