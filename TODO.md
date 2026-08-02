# TODO

Checklist de evolução do MVP para produto. Itens derivados de
[docs/regras-de-negocio.md](docs/regras-de-negocio.md) e da leitura do código
atual (backend Go, frontend React/PWA, infra Docker).

## 1. Autenticação & Autorização (bloqueador para produção)

- [x] Login (usuário/senha) para gestor/recepção e profissional
- [x] Autorização por papel: gestor/recepção e profissional (cliente ganhou login nesta etapa também, ver abaixo)
- [x] Sessão/JWT com expiração e refresh
- [x] Login social (Google, Facebook) para quem tem e-mail — também funciona como recuperação de acesso de staff (entrar com o mesmo e-mail da conta) e como autocadastro de cliente na primeira vez
- [x] Cliente sem e-mail: barbeiro cadastra nome + celular e concede acesso (`POST /customers/{id}/credentials`); senha temporária enviada por SMS/WhatsApp via Zenvia
- [x] Rate limiting só no login por celular (sem e-mail): 3 tentativas erradas bloqueiam a conta (`423 account_locked`)
- [x] Recuperação por celular: novo número informado recebe senha nova por SMS/WhatsApp e desbloqueia a conta
- [ ] Zenvia real: validar o payload da API contra a documentação atual e testar com uma conta de verdade (implementado sem credenciais reais, só o fallback de log foi testado)
- [ ] Credenciais reais de Google/Facebook OAuth (implementado e testado com `oauth_not_configured`; falta testar o fluxo completo com um app registrado em cada provedor)
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
