# API HTTP

Base: `/api/v1`. Corpos e respostas usam JSON.

## Autenticação

Todas as rotas abaixo, exceto as marcadas como "pública", exigem um access
token JWT no cabeçalho `Authorization: Bearer <token>`.

Há três formas de autenticar, dependendo de quem tem e-mail e quem não tem:

1. **E-mail + senha** — `POST /auth/login` com `{"email","password"}`. Usado
   hoje pelo gestor/recepção seed. Sem rate limiting (ver item 3).
2. **Login social (Google/Facebook)** — `GET /auth/google/start` ou
   `GET /auth/facebook/start` redireciona ao provedor; o callback devolve o
   navegador para `FRONTEND_URL` com `#access_token=...&refresh_token=...`.
   Resolve conta por e-mail: se já existir um usuário (staff ou cliente) com
   aquele e-mail, apenas vincula a conta social a ele — isso também serve
   como recuperação de acesso de staff sem precisar de "esqueci minha senha".
   Se não existir, autocadastra um cliente novo. Responde `501
   oauth_not_configured` se as credenciais do provedor não estiverem nas
   variáveis de ambiente.
3. **Celular + senha (cliente sem e-mail)** — `POST /auth/login` com
   `{"phone","password"}`. Rate limited: 3 tentativas erradas bloqueiam a
   conta (`423 account_locked`); o desbloqueio só acontece pedindo uma senha
   nova em `POST /auth/recover` (`{"phone","channel"}`), enviada por
   SMS/WhatsApp. O barbeiro concede esse acesso pela primeira vez em
   `POST /customers/{id}/credentials` (mesmo corpo), que gera e envia a
   senha inicial.

| Método | Rota | Descrição | Autenticação |
|---|---|---|---|
| POST | `/auth/login` | Login com e-mail+senha ou celular+senha | pública |
| POST | `/auth/refresh` | Troca um refresh token válido por um novo access token | pública |
| POST | `/auth/recover` | Envia senha nova por SMS/WhatsApp a um celular cadastrado | pública |
| GET | `/auth/google/start`, `/auth/facebook/start` | Redireciona ao provedor OAuth | pública |
| GET | `/auth/google/callback`, `/auth/facebook/callback` | Callback OAuth; redireciona ao frontend com tokens | pública |
| GET | `/auth/me` | Retorna o usuário autenticado | qualquer papel |
| GET | `/services` | Lista serviços | qualquer papel |
| POST | `/services` | Cria serviço | `manager` |
| GET | `/professionals` | Lista profissionais | qualquer papel |
| POST | `/professionals` | Cria profissional | `manager` |
| GET | `/customers` | Lista clientes | qualquer papel |
| POST | `/customers` | Cria cliente | qualquer papel |
| POST | `/customers/{id}/credentials` | Concede/renova acesso por celular a um cliente | `manager` ou `professional` |
| GET | `/appointments?from=&to=` | Lista agenda no período | qualquer papel |
| POST | `/appointments` | Cria agendamento | qualquer papel |
| PATCH | `/appointments/{id}/status` | Altera estado | qualquer papel (profissional só no próprio agendamento) |

Exemplo de login por e-mail:

```json
{ "email": "admin@barberflow.local", "password": "change-me" }
```

Exemplo de login por celular:

```json
{ "phone": "+5511988887777", "password": "089027" }
```

Resposta (ambos os casos):

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "user": { "id": "uuid", "name": "Administrador", "email": "admin@barberflow.local", "role": "manager", "active": true }
}
```

O usuário gestor inicial (`admin@barberflow.local` / `change-me`, criado pela
migração `002_users.sql`) deve trocar a senha assim que possível — use o
login social com o mesmo e-mail para trocar de método de acesso sem depender
da senha atual; até lá, trate essa credencial como sensível e restrinja o
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
