---
name: workflow
description: Conventions for working on barbershopflow — git branch/commit workflow, backend/frontend test patterns, and the verification bar before calling a change done. Load before starting implementation work, before committing, or before writing new tests in this repo.
---

# Convenções do barbershopflow

Premissas confirmadas neste repositório — não são regras de negócio (essas
estão em `docs/regras-de-negocio.md` e `docs/api.md`, e o `README.md` tem o
quick start), são sobre *como* trabalhar aqui.

## 1. Git: uma branch, um commit, quem mescla é o usuário

- Uma tarefa = uma branch = um commit. Nunca `git commit --amend` —
  correção vira commit novo.
- Nome de branch: `feature/<kebab-case>` ou `fix/<kebab-case>`, baseado em
  `main` (a menos que a tarefa dependa de algo que só existe numa branch
  irmã ainda não mesclada — nesse caso, branch a partir dela e diga isso
  explicitamente).
- Mensagem de commit, no padrão já usado no histórico real do projeto
  (Conventional Commits, em português):
  - Título: `type(scope): descrição curta` — tipos vistos: `feat`, `fix`,
    `test`, `chore`; escopo é um substantivo de domínio em português
    (`relatorios`, `agendamentos`, `profissionais`, `backend`, `e2e`...),
    não um caminho de arquivo.
  - Corpo: prosa explicando o *porquê* e o que foi implementado — não uma
    narração do diff.
  - Parágrafo final começando com "Testado:" listando a verificação real
    feita (ver seção 3).
  - Fecha com `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`.
- Atualize `TODO.md` no mesmo commit quando a tarefa corresponde a uma
  linha dele — marque `[x]` e reescreva a linha resumindo o que foi
  construído de fato, não só "feito".
- `git push -u origin <branch>` e pare por aí. O usuário mescla pela
  própria UI do GitHub. Nunca abra ou mescle um PR sozinho sem pedido
  explícito.
- Se no meio da implementação uma regra de negócio ficar ambígua ou
  conflitante, pare e pergunte — não assuma e continue.
- Ao mesclar várias branches localmente (só quando pedido explicitamente,
  não é o padrão): `git merge --no-ff` uma de cada vez, rodando a barra de
  verificação (seção 3) depois de cada merge, não só no final. Conflitos
  em `TODO.md`/`docs/api.md`/`server.go`/`App.tsx` entre branches
  independentes quase sempre se resolvem mantendo os dois lados
  (concatenando), exceto quando há choque real de assinatura/nome — aí é
  reconciliar de verdade. Migrations com número duplicado entre branches
  irmãs: renumerar em commit próprio na branch, antes de mesclar, nunca
  reescrevendo o commit original.

## 2. Padrões de teste já em uso

### Backend (Go — stdlib `testing`, sem testify)

- Estilo table-driven com subtests (`t.Run`).
- Handlers HTTP (`backend/internal/http`): um `fakeStore` compartilhado
  (`store_fake_test.go`) implementa a interface `Store` inteira em
  memória; injeção de erro por teste via `store.errs["NomeDoMétodo"] =
  domain.ErrX`. Os testes sobem o servidor real via `New(store, cfg)` —
  nunca um `server{}` montado à mão — para exercitar roteamento e
  middleware junto.
- Testes de integração do repository (`backend/internal/repository`):
  testcontainers-go sobe um Postgres real (`postgres:17-alpine`),
  aplicando as migrations de verdade via `internal/database.Migrate`.
  Gate por `testing.Short()`, não build tag — então **`go test ./...`
  agora exige Docker rodando**; `go test -short ./...` continua sendo o
  ciclo rápido sem Docker.

### Frontend (Vitest 3.x + Testing Library + Playwright)

- Testes de componente ficam ao lado do código-fonte
  (`Foo.tsx`/`Foo.test.tsx` no mesmo diretório), mockando o módulo
  `./api` inteiro (`vi.mock('./api')`) em vez de interceptar `fetch` cru.
- `vite.config.ts` usa `globals: false` — importe `describe`/`it`/`expect`
  explicitamente de `'vitest'`, igual aos arquivos já existentes. Cleanup
  do DOM entre testes é manual, via `afterEach(cleanup)` em
  `frontend/src/test/setup.ts` (o auto-cleanup do Testing Library só
  registra sozinho quando `globals: true`).
- `test.include` em `vite.config.ts` é restrito a `src/**/*.test.{ts,tsx}`
  — não deixe cair para o default, que também casaria com
  `e2e/*.spec.ts` (que usa `test`/`expect` do `@playwright/test`, não do
  Vitest, e quebra `npm test`).
- E2E de verdade em `frontend/e2e/` (`npm run test:e2e`), rodando contra o
  stack real do `make dev` (frontend `:5173` proxeando `/api` pro backend
  `:8080` com Postgres de verdade) — não um build estático. Sem
  `webServer` no `playwright.config.ts`: os specs assumem que `make dev`
  já está de pé.
  - Login por e-mail (gestor via signup, superadmin via
    `admin@barberflow.local`) é o único caminho automatizável em E2E
    neste projeto — cliente e profissional só logam por celular (senha
    entregue via SMS/WhatsApp, não recuperável num teste) ou OAuth (sem
    provedor configurado em dev). Specs que precisariam de um desses
    papéis testam o que dá pra exercitar pelo lado do gestor/API em vez
    disso.
  - Promover um e-mail a superadmin via `POST /admin/promote` deixa o
    tenant daquele e-mail **sem gestor nenhum** (quebra
    `impersonateTenant`, que busca o primeiro gestor do tenant). Em
    specs que testam promote+impersonate, promova o gestor de um tenant
    *diferente* do que está sendo impersonado.
- Rode `npm run build` (`tsc -b && vite build`), não só `tsc --noEmit` —
  já houve caso real de `tsc --noEmit` deixar passar um call-site
  desatualizado que só o build completo pegou.

## 3. Barra de verificação antes de considerar algo pronto

1. Backend: `cd backend && go build ./... && go vet ./... && go test ./...`
   (ou `go test -short ./...` se só quiser o ciclo rápido sem Docker).
2. Frontend: `cd frontend && npm run build && npm test`.
3. Mudou UI ou um fluxo ponta-a-ponta? `npm run test:e2e` contra o
   `make dev` real, ou estenda `frontend/e2e/` com um spec novo se o
   fluxo for algo que vale a pena reexecutar depois.
4. Adicionou uma migration? Reinicie o container da API (`docker restart
   barberflow-api-1` — ele roda `go run`, **sem hot reload**) e confirme
   o schema aplicado (`docker exec barberflow-db-1 psql -U barbershop -d
   barbershop -c '\d <tabela_nova>'`).
5. Exercite endpoints novos/alterados com `curl` real contra
   `http://localhost:8080/api/v1` — caminho feliz, rejeição de
   auth/role (401/403), casos de validação e conflito.
6. Contas seed disponíveis (senha `change-me` para as duas):
   `admin@barberflow.local` (superadmin) e `gestor@barberflow.local`
   (gestor da barbearia demo).
7. Reverta qualquer mutação incidental feita em dados **seed** durante o
   teste (ex: renomear a barbearia demo de volta). Tenants/clientes
   criados só para verificação podem ficar no banco de dev.

O stack sobe com `make dev` (serviços `barberflow-api-1`,
`barberflow-web-1`, `barberflow-db-1`); se os containers não estiverem no
ar, suba com `make dev` e espere `barberflow-api-1` reportar saudável
antes de testar.
