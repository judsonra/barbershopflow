# Regras de negócio

## Escopo do MVP

### Perfis

- **Cliente:** pessoa atendida, identificada por nome, telefone e e-mail
  opcionais. Autentica com papel `client` de duas formas: login social
  (Google/Facebook) se tiver e-mail — com autocadastro na primeira vez — ou
  celular + senha temporária se não tiver, senha essa enviada por
  SMS/WhatsApp quando o barbeiro concede o acesso. Só enxerga os próprios
  agendamentos (nunca a lista de clientes ou a agenda inteira da barbearia).
  Pode sugerir o próprio horário se a barbearia habilitar o autoagendamento —
  ver seção própria abaixo.
- **Profissional:** barbeiro disponível para receber agendamentos. Autentica
  com papel `professional`, por login social (Google/Facebook) se tiver
  e-mail cadastrado, ou por celular + senha temporária se não tiver — o
  gestor concede esse acesso pela primeira vez (ou renova) em
  `POST /api/v1/professionals/{id}/credentials`, senha enviada por
  SMS/WhatsApp, mesmo mecanismo usado para cliente. Só enxerga a própria
  agenda (`GET /api/v1/appointments` filtra por `professional_id`), só pode
  alterar o status de agendamentos onde é o profissional responsável; também
  pode cadastrar clientes e conceder acesso por celular a eles.
- **Gestor/recepção:** opera catálogo, profissionais, clientes e agenda.
  Autentica com papel `manager`; único papel autorizado a cadastrar serviços e
  profissionais.
- **Superadmin:** operador da plataforma, sem barbearia própria — hoje só
  `admin@barberflow.local` (seed da migração `009_superadmin_seed.sql`).
  Só lista as barbearias e "acessa" uma virando o gestor de verdade dela
  (`POST /api/v1/admin/tenants/{id}/impersonate`); não tem nenhum atalho nos
  demais endpoints, que continuam scoped a um único tenant como sempre. Ver
  [docs/api.md](api.md).

Gestor loga por e-mail e senha (`POST /api/v1/auth/login`) ou por login social
com o mesmo e-mail da conta — que funciona como recuperação de acesso, sem
precisar de "esqueci minha senha". Cliente e profissional sem e-mail logam
por celular e senha, com rate limiting (3 tentativas erradas bloqueiam a
conta) e recuperação via nova senha por SMS/WhatsApp
(`POST /api/v1/auth/recover`). Detalhes em [docs/api.md](api.md). Revogação de
refresh tokens ainda não existe — ver `TODO.md`. Até lá, trate a API
administrativa (rotas de gestor/profissional) como sensível e restrinja seu
acesso a redes confiáveis.

### Multiunidade (tenant)

1. Cada barbearia é um `tenant` isolado: serviços, profissionais, clientes e
   agendamentos de uma barbearia nunca aparecem para outra.
2. Onboarding é self-service — `POST /api/v1/tenants` cria a barbearia e o
   gestor inicial numa única operação, sem precisar de provisionamento manual.
3. E-mail, celular e ids de login social (Google/Facebook) identificam uma
   **identidade** (a pessoa, com sua senha), única globalmente — não por
   barbearia. Uma identidade pode ter um **vínculo** (`membership`) em mais
   de uma barbearia ao mesmo tempo (ex: cliente atendido em duas
   barbearias), cada vínculo com seu próprio papel; a senha é uma só e
   resetar em qualquer lugar vale para todos os vínculos. Login com um
   único vínculo ativo entra direto, sem escolher barbearia, como sempre
   foi; com mais de um vínculo ativo, a API responde com a lista para
   escolher (`POST /auth/select-membership`, ver [docs/api.md](api.md)). A
   única exceção que precisa saber a barbearia de antemão é o autocadastro
   social de um cliente totalmente novo (identidade nova ou identidade
   existente sem vínculo ainda naquela barbearia), que precisa do
   `?tenant=<slug>` da barbearia (ver [docs/api.md](api.md)).

### Serviços

1. Todo serviço tem nome, duração em minutos e preço não negativo.
2. Serviços inativos permanecem no histórico, mas não devem receber novos
   agendamentos.
3. A duração determina o horário final do atendimento.

### Profissionais

1. Todo profissional tem nome e pode estar ativo ou inativo.
2. Profissionais inativos permanecem no histórico, mas não recebem novos
   agendamentos.
3. Especialidades (`professional_services`, `GET`/`PUT
   /professionals/{id}/services`): profissional sem nenhuma especialidade
   cadastrada pode executar qualquer serviço ativo, com o preço/duração
   padrão do serviço (retrocompatível com profissionais criados antes dessa
   tabela existir). A partir da primeira especialidade cadastrada, o
   profissional só pode ser agendado para os serviços marcados (`409
   service_not_offered` para os demais); cada especialidade pode opcionalmente
   sobrescrever o preço e/ou a duração daquele serviço só para aquele
   profissional. Só o gestor define especialidades e preços (não é
   autoatendido pelo profissional, ao contrário da jornada de trabalho).

### Jornada de trabalho e bloqueios

1. Cada profissional pode definir, por dia da semana, um horário inicial e
   final de atendimento (`GET`/`PUT /professionals/{id}/schedule`).
2. Sem nenhum dia configurado, o profissional não tem restrição de horário
   (retrocompatível com profissionais criados antes dessa regra existir).
   Com ao menos um dia configurado, um dia da semana sem horário vira folga
   fixa — nenhum agendamento é aceito nele.
3. Além da jornada recorrente, o profissional pode bloquear períodos
   pontuais (ausência, folga, viagem) com início e fim
   (`GET`/`POST /professionals/{id}/time-off`, `DELETE .../time-off/{id}`).
   Qualquer sobreposição com um bloqueio impede a criação do agendamento,
   mesmo dentro da jornada normal.
4. Um agendamento só é aceito se `[início, fim)` couber inteiramente dentro
   da janela do dia da semana (quando configurada) e não sobrepuser nenhum
   bloqueio. Vale tanto para staff quanto para autoagendamento do cliente.
5. Só o próprio profissional ou o gestor podem editar a jornada e os
   bloqueios de um profissional.

### Clientes

1. Nome é obrigatório.
2. Telefone e e-mail são opcionais.
3. O histórico de atendimentos referencia o cliente e não é apagado em cascata.

### Agendamentos

1. Um agendamento exige cliente, profissional, serviço e início.
2. O fim é calculado como `início + duração do serviço`; a API não confia em um
   fim enviado pelo cliente.
3. Cliente, profissional e serviço precisam existir; profissional e serviço
   precisam estar ativos.
4. Um profissional não pode ter atendimentos sobrepostos nos estados
   `scheduled` ou `confirmed`.
5. Comparação de sobreposição usa intervalo semiaberto `[início, fim)`: um novo
   atendimento pode começar exatamente quando o anterior termina.
6. Estados e transições:

```text
scheduled ──> confirmed ──> completed
     └──────────┴────────> cancelled
```

7. `completed` e `cancelled` são estados finais.
8. Cancelamentos liberam imediatamente o horário.
9. Datas trafegam em ISO 8601/RFC 3339 com fuso. O banco armazena `timestamptz`.

### Autoagendamento

Cada barbearia liga essas duas configurações independentemente (`GET`/`PATCH
/api/v1/tenant`, só `manager`):

1. `self_scheduling_enabled`: se desligado (padrão), só staff cria
   agendamento; cliente que tentar recebe `403`.
2. `auto_confirm_appointments`: com autoagendamento ligado, decide o estado
   inicial do horário sugerido pelo cliente — `confirmed` (reservado e
   confirmado na hora) se ligado, `scheduled` (reservado, mas pendente de
   confirmação do profissional) se desligado. Em ambos os casos o horário já
   bloqueia a agenda (mesma exclusion constraint do item 4 acima), a única
   diferença é se falta ou não uma confirmação manual.
3. Agendamento criado por staff nunca é afetado por esses parâmetros — segue
   sempre `scheduled`, como já era.
4. Cliente só pode agendar para si mesmo: o `customer_id` do corpo da
   requisição é ignorado, a API usa sempre o cliente autenticado.
5. Cliente não pode alterar o status do próprio agendamento (nem confirmar,
   nem cancelar) — isso é papel do profissional/gestor.

## Fora do MVP, mas previsto

- Jornada de trabalho, bloqueios, folgas e feriados.
- Lembretes por WhatsApp/e-mail.
- Sinal, pagamentos, caixa, comissões, cupons e programa de fidelidade.
- Política configurável de cancelamento e no-show.
- LGPD: consentimento, exportação, anonimização e trilha de auditoria.
- Relatórios de ocupação, faturamento e retenção.

## Critérios de aceite do MVP

- Cadastrar e listar serviços, profissionais e clientes.
- Criar, listar por período e alterar o status de agendamentos.
- Rejeitar sobreposição de agenda com resposta HTTP 409.
- Funcionar como PWA instalável, com shell disponível offline.
- Subir integralmente com Docker Compose em dev e produção.
