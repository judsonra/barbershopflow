-- Falta um jeito do profissional cadastrado (so entrada de catalogo ate
-- aqui) ganhar login de verdade. Espelha o fluxo que ja existe pra cliente
-- sem e-mail: gestor concede acesso por celular, senha temporaria chega por
-- SMS/WhatsApp. Precisa de indice unico em professional_id pro upsert
-- (criar na primeira vez, regenerar senha nas seguintes) funcionar - so
-- users.customer_id tinha esse indice ate agora.
CREATE UNIQUE INDEX users_professional_id_idx ON users (professional_id) WHERE professional_id IS NOT NULL;
