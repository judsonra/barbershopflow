-- Política de cancelamento: prazo mínimo (em horas) antes do início do
-- agendamento para que o próprio cliente possa cancelá-lo. Zero (padrão)
-- significa sem restrição de prazo - cliente pode cancelar a qualquer
-- momento antes do início. Staff/profissional nunca são restringidos por
-- este prazo, só o autocancelamento do cliente.
ALTER TABLE tenants ADD COLUMN cancellation_window_hours integer NOT NULL DEFAULT 0 CHECK (cancellation_window_hours >= 0);
