# API HTTP

Base: `/api/v1`. Corpos e respostas usam JSON.

## Autenticação

Todas as rotas abaixo, exceto as marcadas como "pública", exigem um access
token JWT no cabeçalho `Authorization: Bearer <token>`. Toda a API é
multi-tenant: o token carrega `tenant_id` e cada gestor/profissional/cliente
só enxerga dados da própria barbearia — ver "Multi-tenant" abaixo.

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
| POST | `/tenants` | Cria uma barbearia nova + gestor inicial; já retorna tokens (auto-login) | pública |
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
| GET | `/professionals/{id}/schedule` | Jornada semanal do profissional | qualquer papel |
| PUT | `/professionals/{id}/schedule` | Substitui a jornada semanal inteira | `manager` ou o próprio `professional` |
| GET | `/professionals/{id}/time-off?from=&to=` | Lista bloqueios (ausência/folga/viagem) no período | qualquer papel |
| POST | `/professionals/{id}/time-off` | Cria um bloqueio | `manager` ou o próprio `professional` |
| DELETE | `/professionals/{id}/time-off/{blockId}` | Remove um bloqueio | `manager` ou o próprio `professional` |
| GET | `/customers` | Lista clientes | `manager` ou `professional` |
| POST | `/customers` | Cria cliente | `manager` ou `professional` |
| POST | `/customers/{id}/credentials` | Concede/renova acesso por celular a um cliente | `manager` ou `professional` |
| GET | `/tenant` | Configurações da própria barbearia | qualquer papel |
| PATCH | `/tenant` | Liga/desliga autoagendamento e auto-confirmação | `manager` |
| GET | `/admin/tenants` | Lista todas as barbearias da plataforma | `superadmin` |
| POST | `/admin/tenants/{id}/impersonate` | Vira o gestor daquela barbearia (novo access/refresh token) | `superadmin` |
| GET | `/appointments?from=&to=` | Lista agenda no período (`client` só vê os próprios) | qualquer papel |
| POST | `/appointments` | Cria agendamento; `client` só se autoagendamento estiver ligado, ver "Autoagendamento" | qualquer papel |
| PATCH | `/appointments/{id}/status` | Altera estado | `manager`/`professional` (profissional só no próprio agendamento); `client` não pode |

Exemplo de login por e-mail:

```json
{ "email": "gestor@barberflow.local", "password": "change-me" }
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
  "user": { "id": "uuid", "name": "Gestor Demo", "email": "gestor@barberflow.local", "role": "manager", "active": true }
}
```

O gestor inicial da barbearia demo (`gestor@barberflow.local` / `change-me`)
e o superadmin da plataforma (`admin@barberflow.local` / `change-me`, ver
seção "Acesso administrativo" abaixo), ambos criados pelas migrações
`002_users.sql`/`009_superadmin_seed.sql`, devem trocar a senha assim que
possível — use o login social com o mesmo e-mail para trocar de método de
acesso sem depender da senha atual; até lá, trate essas credenciais como
sensíveis e restrinja o acesso à API a redes confiáveis.

## Multi-tenant

Cada barbearia é um `tenant`, criado via `POST /tenants`:

```json
{
  "tenant_name": "Barbearia do Zé",
  "slug": "barbearia-do-ze",
  "manager_name": "Zé",
  "email": "ze@barbeariadoze.com",
  "password": "senha-forte"
}
```

Resposta igual à de login (`access_token`, `refresh_token`, `user`), já
autenticado como gestor da barbearia recém-criada. `slug` só aceita letras
minúsculas, números e hífen, e precisa ser único (`409 conflict`, assim como
e-mail duplicado). Login social (Google/Facebook) de um cliente totalmente
novo — sem conta existente com aquele e-mail — também precisa saber em qual
barbearia se cadastrar: passe `?tenant=<slug>` em `/auth/google/start` ou
`/auth/facebook/start`.

Identificadores de login (e-mail, celular, ids do Google/Facebook) são
globalmente únicos entre barbearias — uma pessoa pertence a uma única
barbearia por vez. O isolamento que importa é o dos dados de negócio
(serviços, profissionais, clientes, agendamentos), sempre filtrados pelo
`tenant_id` do token.

### Acesso administrativo (superadmin)

O papel `superadmin` é o operador da plataforma, sem barbearia própria —
hoje só a conta seed `admin@barberflow.local` tem esse papel (migração
`009_superadmin_seed.sql`). Ele não opera nenhum endpoint de dados
diretamente: `GET /admin/tenants` lista todas as barbearias e
`POST /admin/tenants/{id}/impersonate` devolve um access/refresh token novo
para o **gestor de verdade** daquela barbearia (mesmo formato de resposta do
login). A partir daí, todo o resto da API funciona exatamente igual, sem
nenhuma checagem especial de superadmin em nenhum outro handler.

Isso significa que uma barbearia sem nenhum gestor não pode ser
impersonada — `impersonate` responde `404`. Toda barbearia criada via
`POST /tenants` já vem com um gestor por construção, então isso só seria um
problema em caso de remoção manual de dados.

## Autoagendamento

`GET /tenant` retorna as configurações da barbearia:

```json
{ "id": "uuid", "name": "...", "slug": "...", "self_scheduling_enabled": false, "auto_confirm_appointments": false, "active": true }
```

`PATCH /tenant` (só `manager`) liga/desliga os dois parâmetros independentes:

```json
{ "self_scheduling_enabled": true, "auto_confirm_appointments": false }
```

- `self_scheduling_enabled = false`: só staff cria agendamento; `client` que
  tentar recebe `403 forbidden`.
- `self_scheduling_enabled = true`: `client` pode criar agendamento para si
  mesmo (o `customer_id` enviado é ignorado — a API usa sempre o cliente do
  token). O estado inicial depende de `auto_confirm_appointments`:
  `confirmed` se ligado, `scheduled` (pendente) se desligado. Agendamento
  criado por staff sempre começa `scheduled`, independente dessas flags.
- Em qualquer caso o horário já bloqueia a agenda assim que criado (mesma
  regra de sobreposição do item 4 de "Agendamentos" em
  [regras-de-negocio.md](regras-de-negocio.md)) — só quem confirma
  manualmente é que muda.

`POST /professionals` aceita `{"name","phone","email","cpf"}` — só `name`
é obrigatório. `email` e `cpf` são únicos por barbearia quando informados
(`409 conflict` em caso de repetição) e validados no servidor: e-mail por
formato, CPF pelos dígitos verificadores (`400 validation_error` se
inválido). CPF pode ser enviado formatado (`111.444.777-35`) ou só dígitos —
o servidor normaliza antes de salvar.

## Jornada de trabalho e bloqueios

`PUT /professionals/{id}/schedule` substitui a jornada semanal inteira
(dias ausentes do array viram folga fixa, se houver ao menos um dia
presente):

```json
{
  "entries": [
    { "weekday": 1, "start_minute": 540, "end_minute": 1080 },
    { "weekday": 2, "start_minute": 540, "end_minute": 1080 }
  ]
}
```

`weekday` segue a convenção do Go (`time.Weekday`): 0=domingo..6=sábado.
`start_minute`/`end_minute` são minutos desde a meia-noite (540 = 09:00,
1080 = 18:00), no fuso em que os agendamentos desse profissional são
criados. Sem nenhuma entrada, o profissional não tem restrição de horário.

`POST /professionals/{id}/time-off` cria um bloqueio pontual:

```json
{ "starts_at": "2026-08-10T00:00:00-03:00", "ends_at": "2026-08-17T00:00:00-03:00", "reason": "Viagem" }
```

`POST /appointments` responde `409 outside_working_hours` se o horário cair
fora da jornada do dia, e `409 time_blocked` se sobrepuser um bloqueio.

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
