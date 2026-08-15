# BarberFlow

PWA mobile-first para gestão de barbearias, com React no frontend, API Go e
PostgreSQL. O projeto traz ambientes Docker separados para desenvolvimento e
produção.

## Início rápido

```bash
cp .env.example .env
make dev
```

- Aplicação: http://localhost:5173
- API: http://localhost:8080/api/v1
- Health check: http://localhost:8080/health

Para encerrar: `make down`. Para executar em produção local: `make prod`.

## Acessos de desenvolvimento

Usuários criados pelas migrações de seed (apenas em dev/demo):

| Papel      | E-mail                     | Senha       |
| ---------- | -------------------------- | ----------- |
| Superadmin | `admin@barberflow.local`   | `change-me` |
| Gestor     | `gestor@barberflow.local`  | `change-me` |

Troque essas senhas imediatamente em qualquer ambiente que não seja local.

## Comandos

```bash
make help          # lista os comandos
make dev           # ambiente com hot reload no frontend
make prod          # build e execução da imagem de produção
make test          # testes do backend e frontend
make build         # builds locais
make logs          # logs do ambiente ativo
make clean         # remove containers e volumes (pede confirmação)
```

## Arquitetura

```text
React PWA ──HTTP/JSON──> API Go ──SQL──> PostgreSQL
     └──── produção: Nginx encaminha /api para a API
```

O frontend é uma SPA instalável e responsiva. A API usa `net/http`, PostgreSQL
via `pgx`, migrações versionadas e regras de domínio isoladas do transporte.

Documentação:

- [Regras de negócio](docs/regras-de-negocio.md)
- [API](docs/api.md)

## Variáveis de ambiente

Consulte `.env.example`. Em produção, altere obrigatoriamente as credenciais do
banco e configure `CORS_ORIGINS` com as origens reais.

## Estrutura

```text
backend/       API Go e migrações
frontend/      React + Vite + PWA
deploy/        configuração do Nginx
docs/          decisões e regras de negócio
compose*.yml   ambientes base, dev e produção
```
