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
   Resolve conta por e-mail: se já existir uma identidade (staff ou cliente)
   com aquele e-mail, vincula a conta social a ela — isso também serve como
   recuperação de acesso de staff sem precisar de "esqueci minha senha". Se
   a identidade existe mas ainda não tem vínculo na barbearia do
   `?tenant=<slug>` da URL (ver "Multi-tenant" abaixo), cria o vínculo novo
   ali em vez de logar na conta de outra barbearia. Se a identidade não
   existe, autocadastra um cliente novo. Responde `501 oauth_not_configured`
   se as credenciais do provedor não estiverem nas variáveis de ambiente.
3. **Celular + senha (cliente ou profissional sem e-mail)** — `POST
   /auth/login` com `{"phone","password"}`. Rate limited: 3 tentativas
   erradas bloqueiam a conta (`423 account_locked`); o desbloqueio só
   acontece pedindo uma senha nova em `POST /auth/recover`
   (`{"phone","channel"}`), enviada por SMS/WhatsApp — vale tanto para
   cliente quanto para profissional. O gestor concede esse acesso pela
   primeira vez (ou renova) em `POST /customers/{id}/credentials` ou
   `POST /professionals/{id}/credentials` (mesmo corpo), que gera e envia a
   senha inicial. Profissional autenticado por celular só enxerga a própria
   agenda (ver `GET /appointments` abaixo).

Identidade e vínculo são coisas separadas: a mesma pessoa (mesmo e-mail,
celular ou login social) pode ter um vínculo (`membership`) em mais de uma
barbearia, com **uma senha só** — resetar em `POST /auth/recover` vale para
todas. Por isso `POST /auth/login` (nas formas 1 e 3) e o callback OAuth da
forma 2 têm duas respostas possíveis:

- **Um vínculo ativo** (caso comum, sem mudança de comportamento): resposta
  de token normal — ver "Resposta" abaixo.
- **Mais de um vínculo ativo**: em vez de tokens, `200` com
  `{"preauth_token": "...", "memberships": [{"membership_id", "tenant_id",
  "tenant_name", "tenant_slug", "role"}, ...]}`. O cliente escolhe uma
  barbearia e chama `POST /auth/select-membership` com
  `{"preauth_token", "membership_id"}` para completar o login (mesma
  resposta de token normal). `preauth_token` expira em 5 minutos e só serve
  para essa troca — não autentica nenhum outro endpoint. No callback OAuth
  (forma 2), o caso ambíguo redireciona com
  `#preauth_token=...&needs_selection=1` em vez de `access_token`.

Concessão de acesso (`POST /customers/{id}/credentials` e
`POST /professionals/{id}/credentials`) segue a mesma lógica: se o celular
informado já é uma identidade conhecida (staff/cliente de outra barbearia),
a chamada só cria o vínculo novo — **não** gera nem envia senha nova, pra
não invalidar a sessão que a pessoa já tinha em outro lugar. Só gera/envia
senha quando a identidade é realmente nova.

| Método | Rota | Descrição | Autenticação |
|---|---|---|---|
| GET | `/tenants/availability?name=` | Checa se o nome da barbearia está disponível | pública |
| POST | `/tenants` | Cria uma barbearia nova + gestor inicial; já retorna tokens (auto-login) | pública |
| POST | `/auth/login` | Login com e-mail+senha ou celular+senha | pública |
| POST | `/auth/refresh` | Troca um refresh token válido por um novo access token | pública |
| POST | `/auth/recover` | Envia senha nova por SMS/WhatsApp a um celular cadastrado (vale em todas as barbearias da identidade) | pública |
| POST | `/auth/select-membership` | Completa o login quando a identidade tem mais de um vínculo ativo | pública |
| GET | `/auth/google/start`, `/auth/facebook/start` | Redireciona ao provedor OAuth | pública |
| GET | `/auth/google/callback`, `/auth/facebook/callback` | Callback OAuth; redireciona ao frontend com tokens | pública |
| GET | `/auth/me` | Retorna o usuário autenticado | qualquer papel |
| GET | `/services` | Lista serviços | qualquer papel |
| POST | `/services` | Cria serviço | `manager` |
| PATCH | `/services/{id}` | Atualiza nome/duração/preço/ativo (usado também para desativar/reativar) | `manager` |
| GET | `/professionals` | Lista profissionais | qualquer papel |
| POST | `/professionals` | Cria profissional | `manager` |
| PATCH | `/professionals/{id}` | Atualiza dados/ativo do profissional (usado também para desativar/reativar) | `manager` |
| GET | `/professionals/{id}/schedule` | Jornada semanal do profissional | qualquer papel |
| PUT | `/professionals/{id}/schedule` | Substitui a jornada semanal inteira | `manager` ou o próprio `professional` |
| GET | `/professionals/{id}/time-off?from=&to=` | Lista bloqueios (ausência/folga/viagem) no período | qualquer papel |
| POST | `/professionals/{id}/time-off` | Cria um bloqueio | `manager` ou o próprio `professional` |
| DELETE | `/professionals/{id}/time-off/{blockId}` | Remove um bloqueio | `manager` ou o próprio `professional` |
| POST | `/professionals/{id}/credentials` | Concede/renova acesso por celular a um profissional | `manager` |
| GET | `/customers` | Lista clientes | `manager` ou `professional` |
| POST | `/customers` | Cria cliente | `manager` ou `professional` |
| PATCH | `/customers/{id}` | Atualiza dados/ativo do cliente (usado também para desativar/reativar) | `manager` ou `professional` |
| POST | `/customers/{id}/credentials` | Concede/renova acesso por celular a um cliente | `manager` ou `professional` |
| GET | `/tenant` | Configurações da própria barbearia | qualquer papel |
| PATCH | `/tenant` | Liga/desliga autoagendamento e auto-confirmação | `manager` |
| GET | `/admin/tenants` | Lista todas as barbearias da plataforma | `superadmin` |
| POST | `/admin/tenants/{id}/impersonate` | Vira o gestor daquela barbearia (novo access/refresh token) | `superadmin` |
| GET | `/admin/customers?q=` | Busca clientes por nome/celular/e-mail em todas as barbearias | `superadmin` |
| POST | `/admin/promote` | Promove outra identidade a `superadmin` pelo e-mail | `superadmin` |
| GET | `/admin/audit` | Trilha de auditoria de impersonate (quem, qual barbearia, quando) | `superadmin` |
| GET | `/appointments?from=&to=` | Lista agenda no período (`client` só vê os próprios; `professional` só vê os da própria agenda) | qualquer papel |
| POST | `/appointments` | Cria agendamento; `client` só se autoagendamento estiver ligado, ver "Autoagendamento" | qualquer papel |
| PATCH | `/appointments/{id}/status` | Altera estado | `manager`/`professional` (profissional só no próprio agendamento); `client` não pode |
| GET | `/reports?from=&to=` | Ocupação, faturamento e retenção agregados no período (padrão: mês corrente) | `manager` |

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
autenticado como gestor da barbearia recém-criada. `tenant_name` também
precisa ser único (case-insensitive) — `GET /tenants/availability?name=`
checa em tempo real, mas a checagem definitiva é sempre no `POST` (`409
conflict`, assim como `slug`/e-mail duplicado). `slug` só aceita letras
minúsculas, números e hífen. Login social (Google/Facebook) de um cliente totalmente
novo — sem conta existente com aquele e-mail — também precisa saber em qual
barbearia se cadastrar: passe `?tenant=<slug>` em `/auth/google/start` ou
`/auth/facebook/start`.

Identificadores de login (e-mail, celular, ids do Google/Facebook) são
globalmente únicos entre barbearias — cada um identifica uma **identidade**
única (uma pessoa, uma senha). Uma identidade pode ter um **vínculo**
(`membership`) em mais de uma barbearia ao mesmo tempo, cada um com seu
próprio papel (`role`) — ver "Autenticação" acima para o fluxo de login
quando há mais de um vínculo. O isolamento que importa é o dos dados de
negócio (serviços, profissionais, clientes, agendamentos), sempre filtrados
pelo `tenant_id` do token, que identifica o vínculo escolhido, não a
identidade.

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

Todo `impersonate` bem-sucedido grava uma linha em `impersonation_audits`
(migração `015_impersonation_audit.sql`), além do log de texto que já
existia — a gravação é best-effort (falhar não impede o acesso). `GET
/admin/audit` lista essa trilha, mais recente primeiro, com quem
impersonou (`actor_name`/`actor_email`) e qual barbearia
(`tenant_id`/`tenant_name`/`tenant_slug`) foi acessada em cada linha;
limitado a 200 linhas — é uma trilha pra revisar, não um relatório
paginado.

A única exceção deliberada ao isolamento por tenant é `GET
/admin/customers?q=`: busca clientes por nome, celular ou e-mail (`ILIKE`,
case-insensitive) em **todas** as barbearias de uma vez, retornando também
`tenant_id`/`tenant_name`/`tenant_slug` de cada resultado — serve pra achar
rápido em qual barbearia um cliente específico está cadastrado, sem
precisar impersonar barbearia por barbearia. `q` vazio devolve lista vazia
em vez de despejar a base inteira; resultado limitado a 50 linhas (é um
atalho de busca, não um relatório). Não devolve dado de negócio (serviços,
profissionais, agendamentos) de nenhuma barbearia — pra isso, ainda é
preciso impersonar.

`POST /admin/promote` com `{"email": "..."}` substitui a promoção manual
via SQL direto que era a única forma de criar um superadmin até agora:
acha a identidade pelo e-mail e vira o `role` do vínculo (`membership`)
dela pra `superadmin`. Exige que a identidade tenha **exatamente um**
vínculo — com mais de um (ex: um cliente com conta em duas barbearias),
não dá pra saber qual barbearia deveria virar o vínculo (tenant-agnóstico,
na prática) do novo superadmin, então a API responde `409
ambiguous_identity` em vez de chutar; ainda dá pra resolver via SQL direto
nesse caso, como antes desse endpoint existir. `404` se o e-mail não
corresponder a nenhuma identidade, ou se a identidade não tiver nenhum
vínculo ativo.

## Autoagendamento

`GET /tenant` retorna as configurações da barbearia:

```json
{ "id": "uuid", "name": "...", "slug": "...", "self_scheduling_enabled": false, "auto_confirm_appointments": false, "active": true }
```

`PATCH /services/{id}` (só `manager`) substitui o serviço inteiro — não há
merge parcial, o corpo sempre traz `name`, `duration_minutes`, `price_cents` e
`active`. "Excluir" um serviço é um soft-delete: manda `active: false` em vez
de remover a linha, porque `appointments.service_id` é uma FK `NOT NULL` sem
`ON DELETE CASCADE` e um DELETE físico falharia assim que o serviço tivesse
qualquer agendamento no histórico.

`PATCH /professionals/{id}` (só `manager`) segue o mesmo contrato: substitui
o profissional inteiro (`name`, `phone`, `email`, `cpf`, `active`), sem merge
parcial, e "Excluir" também é soft-delete via `active: false` — mesma razão
de FK, agora em `appointments.professional_id`.

`PATCH /customers/{id}` (`manager` ou `professional`) segue o mesmo contrato:
substitui o cliente inteiro (`name`, `phone`, `email`, `active`), sem merge
parcial, e "Excluir" também é soft-delete via `active: false` — mesma razão
de FK, agora em `appointments.customer_id`.

`PATCH /tenant` (só `manager`) segue o mesmo contrato de full-replace dos
outros PATCH: substitui `name`, `slug`, `self_scheduling_enabled` e
`auto_confirm_appointments` de uma vez, sem merge parcial — a tela de
configurações que só mexe nos dois parâmetros de agendamento reenvia o
`name`/`slug` atuais sem alteração.

```json
{ "name": "Barbearia do Zé", "slug": "barbearia-do-ze", "self_scheduling_enabled": true, "auto_confirm_appointments": false }
```

`name` não pode ser vazio e precisa ser único (case-insensitive, `409
conflict` se repetido — mesma checagem de `POST /tenants`, `GET
/tenants/availability?name=` pode ser reaproveitado pra checar em tempo
real antes de salvar). `slug` segue o mesmo padrão do cadastro (letras
minúsculas, números e hífen, único, `409` se repetido). Trocar o slug
invalida qualquer link de autocadastro social (`?tenant=<slug>`) já
compartilhado com o slug antigo.

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

## Relatórios

`GET /reports?from=&to=` (só `manager`; sem os parâmetros, padrão é o mês
corrente):

```json
{
  "from": "2026-08-01T00:00:00Z", "to": "2026-09-01T00:00:00Z",
  "occupancy": {
    "overall_rate": 0.42,
    "by_professional": [{ "professional_id": "uuid", "professional_name": "Rafael", "available_minutes": 4800, "booked_minutes": 2016, "rate": 0.42 }]
  },
  "revenue": {
    "total_cents": 350000,
    "by_professional": [{ "professional_id": "uuid", "professional_name": "Rafael", "total_cents": 350000 }]
  },
  "retention": { "total_customers": 20, "returning_customers": 8, "rate": 0.4 }
}
```

`by_professional` só lista profissionais com dado no período (ocupação:
com jornada configurada; faturamento: com pelo menos um `completed`) —
sem entrada não é zero, é "não se aplica".

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
