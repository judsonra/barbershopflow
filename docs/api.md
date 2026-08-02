# API HTTP

Base: `/api/v1`. Corpos e respostas usam JSON.

## Autenticação

Todas as rotas abaixo, exceto `/health`, `/auth/login` e `/auth/refresh`,
exigem um access token JWT no cabeçalho `Authorization: Bearer <token>`.

| Método | Rota | Descrição | Autenticação |
|---|---|---|---|
| POST | `/auth/login` | Login com e-mail e senha; retorna access e refresh token | pública |
| POST | `/auth/refresh` | Troca um refresh token válido por um novo access token | pública |
| GET | `/auth/me` | Retorna o usuário autenticado | qualquer papel |
| GET | `/services` | Lista serviços | qualquer papel |
| POST | `/services` | Cria serviço | `manager` |
| GET | `/professionals` | Lista profissionais | qualquer papel |
| POST | `/professionals` | Cria profissional | `manager` |
| GET | `/customers` | Lista clientes | qualquer papel |
| POST | `/customers` | Cria cliente | qualquer papel |
| GET | `/appointments?from=&to=` | Lista agenda no período | qualquer papel |
| POST | `/appointments` | Cria agendamento | qualquer papel |
| PATCH | `/appointments/{id}/status` | Altera estado | qualquer papel (profissional só no próprio agendamento) |

Exemplo de login:

```json
{ "email": "admin@barberflow.local", "password": "change-me" }
```

Resposta:

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "user": { "id": "uuid", "name": "Administrador", "email": "admin@barberflow.local", "role": "manager", "active": true }
}
```

O usuário gestor inicial (`admin@barberflow.local` / `change-me`, criado pela
migração `002_users.sql`) deve ter a senha trocada assim que o fluxo de troca
de senha existir; até lá, trate essa credencial como sensível e restrinja o
acesso à API a redes confiáveis.

Exemplo de agendamento:

```json
{
  "customer_id": "uuid",
  "professional_id": "uuid",
  "service_id": "uuid",
  "starts_at": "2026-08-01T14:00:00-03:00",
  "notes": "Preferência por máquina 1"
}
```

Erros seguem o formato:

```json
{ "error": { "code": "schedule_conflict", "message": "..." } }
```
