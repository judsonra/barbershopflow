# TODO

Checklist de evolução do MVP para produto. Itens derivados de
[docs/regras-de-negocio.md](docs/regras-de-negocio.md) e da leitura do código
atual (backend Go, frontend React/PWA, infra Docker).

## 1. Autenticação & Autorização (bloqueador para produção)

- [x] Login (usuário/senha) para gestor/recepção e profissional
- [x] Autorização por papel: gestor/recepção e profissional (cliente ainda não autentica — depende do autoagendamento público)
- [x] Sessão/JWT com expiração e refresh
- [ ] Recuperação de senha (fluxo de e-mail)
- [ ] Rate limiting em endpoints de autenticação
- [ ] Até o login existir, manter a API restrita a rede confiável (risco atual documentado em `docs/regras-de-negocio.md`)

## 2. Multi-tenant / multiunidade

- [ ] Adicionar `tenant_id` (barbearia) em `services`, `professionals`, `customers`, `appointments`
- [ ] Isolar todas as queries de `repository.go` por tenant
- [ ] Incluir `tenant_id` na exclusion constraint `appointments_no_overlap` (hoje é global por `professional_id`)
- [ ] Definir fluxo de onboarding de nova barbearia (criação de tenant + usuário admin inicial)
- [ ] Decidir estratégia de isolamento: coluna `tenant_id` compartilhada (recomendado, migração mais simples a partir do schema atual) vs. schema-per-tenant

## 3. Regras de negócio previstas (fora do MVP atual)

- [ ] Jornada de trabalho, bloqueios, folgas e feriados por profissional
- [ ] Especialidades por profissional (hoje qualquer ativo faz qualquer serviço)
- [ ] Preços/durações customizados por profissional
- [ ] Autoagendamento público (cliente cria o próprio horário)
- [ ] Confirmação e lembretes por WhatsApp/e-mail
- [ ] Sinal/pagamento antecipado, caixa, comissões, cupons, fidelidade
- [ ] Política configurável de cancelamento e no-show
- [ ] LGPD: consentimento, exportação de dados, anonimização, trilha de auditoria
- [ ] Relatórios de ocupação, faturamento e retenção

## 4. Melhorias técnicas / infraestrutura

- [ ] CI (lint + testes de backend e frontend) antes de merge
- [ ] Observabilidade: métricas (latência, taxa de erro) e tracing — hoje só há log de texto simples
- [ ] Ampliar cobertura de testes (hoje cobre pouco além do caminho feliz)
- [ ] Estruturar migrações versionadas conforme o schema crescer (tenant, auth, etc.)
- [ ] Paginação em `ListServices`/`ListProfessionals`/`ListCustomers` (hoje retornam tudo sem limite)

## 5. Evolução mobile / PWA → produto instalável completo

- [ ] Push notifications (lembrete de agendamento)
- [ ] Background sync para ações offline (hoje `/api` é `NetworkOnly`, sem fila offline)
- [ ] Avaliar wrapper nativo (Capacitor) se for necessário publicar nas lojas (Play Store/App Store)
- [ ] Ícones/assets completos para todas as plataformas (hoje só `icon.svg`)
