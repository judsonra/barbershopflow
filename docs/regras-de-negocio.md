# Regras de negócio

## Escopo do MVP

### Perfis

- **Cliente:** pessoa atendida, identificada por nome, telefone e e-mail
  opcionais. Autentica com papel `client` de duas formas: login social
  (Google/Facebook) se tiver e-mail — com autocadastro na primeira vez — ou
  celular + senha temporária se não tiver, senha essa enviada por
  SMS/WhatsApp quando o barbeiro concede o acesso. Autoagendamento público
  (o cliente criar o próprio horário) ainda é evolução futura, ver abaixo —
  hoje o login do cliente serve para consultar o próprio histórico.
- **Profissional:** barbeiro disponível para receber agendamentos. Autentica
  com papel `professional`; só pode alterar o status de agendamentos onde é o
  profissional responsável; também pode cadastrar clientes e conceder acesso
  por celular.
- **Gestor/recepção:** opera catálogo, profissionais, clientes e agenda.
  Autentica com papel `manager`; único papel autorizado a cadastrar serviços e
  profissionais.

Staff (gestor/profissional) loga por e-mail e senha (`POST /api/v1/auth/login`)
ou por login social com o mesmo e-mail da conta — que funciona como
recuperação de acesso, sem precisar de "esqueci minha senha". Cliente sem
e-mail loga por celular e senha, com rate limiting (3 tentativas erradas
bloqueiam a conta) e recuperação via nova senha por SMS/WhatsApp
(`POST /api/v1/auth/recover`). Detalhes em [docs/api.md](api.md). Revogação de
refresh tokens ainda não existe — ver `TODO.md`. Até lá, trate a API
administrativa (rotas de gestor/profissional) como sensível e restrinja seu
acesso a redes confiáveis.

### Serviços

1. Todo serviço tem nome, duração em minutos e preço não negativo.
2. Serviços inativos permanecem no histórico, mas não devem receber novos
   agendamentos.
3. A duração determina o horário final do atendimento.

### Profissionais

1. Todo profissional tem nome e pode estar ativo ou inativo.
2. Profissionais inativos permanecem no histórico, mas não recebem novos
   agendamentos.
3. Neste MVP, qualquer profissional ativo pode executar qualquer serviço ativo.
   Uma tabela de especialidades é uma evolução prevista.

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

## Fora do MVP, mas previsto

- Login, autorização por papel e recuperação de senha.
- Multiunidade e isolamento por barbearia (tenant).
- Jornada de trabalho, bloqueios, folgas e feriados.
- Especialidades por profissional e preços/durações customizados.
- Autoagendamento público, confirmação por WhatsApp/e-mail e lembretes.
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
