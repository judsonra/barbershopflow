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

- [x] Adicionar `tenant_id` (barbearia) em `users`, `services`, `professionals`, `customers`, `appointments`
- [x] Isolar todas as queries de `repository.go` por tenant (services/professionals/customers/appointments); login por e-mail/celular/social continua global de propósito — ver `docs/regras-de-negocio.md`
- [x] Incluir `tenant_id` na exclusion constraint `appointments_no_overlap`
- [x] Onboarding self-service: `POST /api/v1/tenants` cria barbearia + gestor inicial numa transação e já faz login (tela "Cadastrar minha barbearia" no frontend)
- [x] Estratégia de isolamento: coluna `tenant_id` compartilhada (decidido, ver migração `005_multi_tenant.sql`)
- [x] Nome da barbearia único (não só o `slug` derivado dele), com checagem de disponibilidade em tempo real no cadastro (`GET /tenants/availability?name=`, debounce de 400ms) — ✓ verde/✗ vermelho ao lado do campo
- [x] Acesso administrativo global: papel `superadmin` (hoje só `admin@barberflow.local`) lista todas as barbearias (`GET /api/v1/admin/tenants`) e "acessa" uma virando o gestor dela (`POST /api/v1/admin/tenants/{id}/impersonate`) — painel próprio no frontend, sem bypass de tenant em nenhum outro endpoint
- [x] Busca global de clientes pelo superadmin: `GET /api/v1/admin/customers?q=` acha cliente por nome/celular/e-mail em todas as barbearias de uma vez (sem precisar impersonar barbearia por barbearia), mostrando de qual barbearia é cada resultado; aba "Clientes" no `AdminPanel` com busca em tempo real (debounce de 400ms) e botão "Acessar como gestor" direto do resultado. Continua sem bypass de tenant nos demais endpoints — só esse lookup é deliberadamente global
- [x] Identidade compartilhada entre barbearias: separa `users` em `identities` (senha/social, única por pessoa) e `memberships` (vínculo por tenant/role/customer_id/professional_id), permitindo um cliente (ou profissional) ter conta em duas barbearias com uma senha só — resetar em uma vale em todas. Migração `014_identity_membership.sql` aplicada e verificada contra o Postgres real; `POST /auth/select-membership` cobre o caso de identidade com múltiplos vínculos ativos (curl e Playwright headless confirmando a tela de escolha de barbearia); concessão de acesso (`POST /customers|professionals/{id}/credentials`) não reenvia senha quando a identidade já existe em outra barbearia; login social cross-tenant também corrigido (identidade existente sem vínculo no `?tenant=` da URL ganha vínculo novo em vez de logar na conta errada — não testado ponta-a-ponta por falta de credenciais OAuth reais, só por leitura do código). `docs/api.md` e `docs/regras-de-negocio.md` atualizados com o novo modelo.
- [ ] Tela de gestão do próprio tenant (trocar nome/slug da barbearia, ver dados da conta)
- [ ] Página de billing/planos, se o produto for monetizado por barbearia
- [ ] Auditoria de impersonate além do log simples atual (quem acessou qual barbearia e quando)
- [ ] Endpoint para promover outro usuário a superadmin (hoje só existe via migração/SQL direto)

## 3. Regras de negócio previstas (fora do MVP atual)

- [x] Jornada de trabalho por profissional (horário inicial/final por dia da semana, `PUT /professionals/{id}/schedule`) e bloqueios pontuais (ausência/folga/viagem, `POST /professionals/{id}/time-off`) — sem jornada configurada = sem restrição (retrocompat); profissional edita a própria, gestor edita qualquer uma
- [ ] Feriados (calendário compartilhado da barbearia, hoje só dá pra bloquear profissional por profissional)
- [x] Especialidades por profissional e preços/durações customizados: tabela `professional_services` (profissional × serviço) com override opcional de preço/duração — `GET`/`PUT /professionals/{id}/services` (`PUT` só `manager`, decisão de preço não é do profissional). Sem nenhuma linha para o profissional, comportamento retrocompatível (qualquer serviço ativo, preço/duração padrão); com ao menos uma linha, só os serviços marcados podem ser agendados para ele (`409 service_not_offered` senão), usando o override quando presente. Seção "Especialidades e preços" na tela de detalhe do profissional (Config → Profissionais)
- [x] Autoagendamento público (cliente cria o próprio horário), com dois parâmetros por barbearia: `self_scheduling_enabled` (liga/desliga) e `auto_confirm_appointments` (auto-confirma ou entra pendente para o profissional confirmar manualmente) — configurável em `PATCH /api/v1/tenant`, aba "Config" no app
- [ ] Lembretes por WhatsApp/e-mail (confirmação em si já existe via auto-confirmação ou confirmação manual acima)
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
- [x] Padronizar campos de hora em 24h: toda hora renderizada pelo app (card da agenda, cabeçalho do dia, lista de bloqueios) usa `hour12: false` explícito, garantido independente do navegador. Os seletores nativos (`type="time"`/`type="datetime-local"`) ganharam `lang="pt-BR"` como dica de melhor esforço, mas o formato exibido por eles (24h vs AM/PM) segue o idioma do navegador/SO do usuário, não a página — confirmado testando com locale forçado em inglês. Decisão registrada: manter o seletor nativo (melhor UX mobile) em vez de trocar por campo de texto com máscara, já que o público-alvo (BR) normalmente já usa o aparelho em pt-BR
- [x] Cadastro de cliente ganhou campo de e-mail no formulário de cadastro rápido (backend já suportava desde o MVP)
- [x] Máscara de celular `(DDD) 9XXXX-XXXX` em todo campo de telefone (login, recuperação, cadastro rápido de cliente); convertido para E.164 (`+55...`) antes de qualquer chamada à API, formato que casa com o que o Zenvia precisa para enviar SMS/WhatsApp de verdade
- [x] Validação explícita de e-mail (JS, não só `type="email"` do navegador) no cadastro de barbearia e no cadastro rápido de cliente — mesmo raciocínio do bug de senha curta: validação só nativa pode não aparecer em navegador/WebView mobile
- [x] Reportado: gestor de barbearia nova (self-service) não tinha nenhuma forma de cadastrar profissionais/serviços pela UI (só existia via API) — aba Config ganhou seções "Serviços" e "Profissionais" com lista + formulário de cadastro
- [x] Reportado: clicar no avatar deslogava na hora, sem confirmação nem tela nenhuma — agora abre uma tela de Perfil (papel, e-mail/celular, status) com um botão "Sair" explícito; corrigido em `AgendaApp` e também no `AdminPanel` do superadmin (mesmo bug, mesmo padrão)
- [x] Cadastro de profissional ganhou e-mail e CPF (ambos opcionais, únicos por barbearia quando informados — `409` se repetido). E-mail validado por formato, CPF pelo dígito verificador (algoritmo padrão, com teste unitário nos dois lados — `backend/internal/domain/models_test.go` e `frontend/src/validation.test.ts`); máscara de CPF (`XXX.XXX.XXX-XX`) ao digitar, mesmo padrão da máscara de celular
- [x] Login do profissional por celular (mesmo fluxo de concessão do cliente, agora espelhado para profissional): gestor clica "Conceder acesso" na lista de Profissionais, informa/confirma o celular, senha temporária é enviada por SMS/WhatsApp (`POST /professionals/{id}/credentials` → `UpsertProfessionalCredentials`, índice único parcial em `users.professional_id`); recuperação por celular (3 tentativas erradas) também vale para profissional agora, não só cliente
- [x] Agenda restrita por papel: `GET /appointments` filtra por `professional_id` quando `role=professional` (cada profissional só vê os próprios atendimentos; gestor continua vendo todos) — testado via curl (isolamento entre dois profissionais) e Playwright (UI mostra "Sua agenda" sem aba Config para o papel profissional)

## 5. Evolução mobile / PWA → produto instalável completo

- [x] Salvar agendamento na agenda do smartphone: link "Google Agenda" e download de `.ics` (Apple Calendar/Outlook/demais apps) em cada agendamento agendado/confirmado
- [ ] Push notifications (lembrete de agendamento)
- [ ] Background sync para ações offline (hoje `/api` é `NetworkOnly`, sem fila offline)
- [ ] Avaliar wrapper nativo (Capacitor) se for necessário publicar nas lojas (Play Store/App Store)
- [ ] Ícones/assets completos para todas as plataformas (hoje só `icon.svg`)
