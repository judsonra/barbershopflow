-- admin@barberflow.local vira o operador global da plataforma em vez de
-- gestor da barbearia demo. Para gerenciar uma barbearia especifica ele
-- usa o fluxo de impersonate (POST /api/v1/admin/tenants/{id}/impersonate),
-- que emite um token normal do gestor daquela barbearia - por isso a demo
-- precisa continuar com um gestor de verdade, senao fica orfa e ninguem
-- consegue impersonate-ar ela.
INSERT INTO users(tenant_id, name, email, password_hash, role)
SELECT tenant_id, 'Gestor Demo', 'gestor@barberflow.local', password_hash, 'manager'
FROM users WHERE email = 'admin@barberflow.local';

UPDATE users SET role = 'superadmin' WHERE email = 'admin@barberflow.local';
