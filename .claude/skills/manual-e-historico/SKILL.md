---
name: manual-e-historico
description: Mantém o manual do usuário final (Docusaurus, em `manual/`) e o diário de bordo `HISTORICO.md` do barbershopflow. Use sempre antes de commitar, abrir PR ou mesclar qualquer mudança destinada à `main` — funcionalidade nova ou removida, regra de negócio, usabilidade, melhoria, bug, teste, infraestrutura ou documentação — mesmo que o pedido não mencione manual nem histórico. Use também quando pedirem para atualizar o manual, registrar algo no histórico ou rastrear o que entrou na `main`.
---

# Manual do usuário e diário de bordo

Toda mudança que entra na `main` deixa rastro em `HISTORICO.md`. Se ela muda o
que o usuário final vê ou pode fazer, também atualiza o manual. As duas
atualizações vão no mesmo commit da tarefa, para entrarem na `main` junto com o
código. Siga a skill `workflow` para branch, commit e push.

## 1. Atualize o HISTORICO.md em toda tarefa

Primeiro **reconcilie** e depois **registre**.

### Reconciliar

O usuário mescla pela interface do GitHub, então o PR e o merge só existem
depois do commit da tarefa. Por isso, cada tarefa completa o que ficou pendente
nas anteriores:

```bash
git fetch origin main
git log --first-parent --format='%h|%ad|%s' --date=format:'%Y-%m-%d %H:%M %z' origin/main
```

Considere os commits mais novos que o último id registrado no arquivo ou, se
nenhuma linha tiver id, mais novos que a base citada no cabeçalho.

- Linha `pendente` cuja branch já entrou na `main`: troque `pendente` pelo id
  real. Não reescreva o restante da linha.
- Commit sem linha correspondente, como um merge feito sem passar por esta
  skill: adicione uma linha com a data e a hora do commit e descreva o que ele
  fez a partir do diff.
- Branch ainda não mesclada: mantenha `pendente`.

O número do PR está no título do merge (`Merge pull request #3 from ...`) ou no
final do título em um squash (`... (#3)`). Se o título não trouxer o número,
consulte `gh pr list --state merged --search <sha>`. Merge sem PR registra só o
hash.

### Registrar

Adicione uma linha ao **final** do arquivo, em ordem cronológica:

```text
- AAAA-MM-DD HH:MM -03 | tipo | o que foi feito | rastreio
```

- **Data e hora:** use `date '+%Y-%m-%d %H:%M %z'` no momento do registro,
  deixando o fuso como `-03`.
- **Tipo:** `feat`, `fix`, `refactor`, `remove`, `test`, `docs`, `infra` ou
  `chore`.
- **O que foi feito:** uma frase curta, sem quebra de linha, dizendo o que mudou
  e não como foi implementado.
- **Rastreio:** `pendente (branch <nome>)` até o merge. Depois, `PR #N · merge
  <hash>`, `squash <hash> · PR #N` ou `merge <hash>`.

Exemplos:

```text
- 2026-11-02 17:18 -03 | fix | Corrige recusa de agendamentos por fuso horário | pendente (branch fix/fuso-agenda)
- 2026-11-03 09:42 -03 | feat | Permite remarcar agendamento | PR #12 · merge a1b2c3d
```

Mantenha as linhas antigas: o arquivo é um diário. Em conflito de merge, mantenha
os dois lados e ordene por data. Se o arquivo não existir, recrie-o com o
cabeçalho do modelo em `HISTORICO.md` na `main`.

## 2. Atualize o manual quando o usuário final for afetado

O manual explica como usar o BarberFlow para gestores, recepção, profissionais e
clientes. `docs/regras-de-negocio.md` e `docs/api.md` continuam sendo os
documentos técnicos e não o substituem.

| A mudança... | Manual |
|---|---|
| Cria ou altera funcionalidade, tela ou fluxo | Atualizar |
| Muda regra de negócio percebida pelo usuário, como prazo, limite, permissão, cálculo ou mensagem | Atualizar |
| Muda usabilidade, como navegação, nome de botão ou ordem de passos | Atualizar |
| Remove funcionalidade | Retirar o conteúdo e explicar a alternativa, se houver |
| Corrige bug e muda o que o usuário vê ou o que o manual descreve | Atualizar |
| Corrige bug sem mudar o uso descrito | Não atualizar |
| Só afeta refatoração, testes, CI, infraestrutura ou documentação técnica | Não atualizar |

Na dúvida, pergunte-se: "um gestor ou cliente perceberia essa diferença?". Se a
resposta for sim, atualize.

### Como escrever

- Escreva em português simples, sem termos técnicos como endpoint, JSON ou
  código HTTP.
- Organize por tarefa do usuário, como "Cancelar um agendamento", em
  `manual/docs/`.
- Diga quem pode fazer a ação, dê o passo a passo e explique os avisos ou
  bloqueios que a pessoa pode encontrar.
- Use os nomes exatos que aparecem na tela e confirme-os no código.
- Descreva o comportamento atual. O que mudou entre versões fica no
  `HISTORICO.md`.

### Se `manual/` ainda não existir

Crie o site na mesma tarefa, com o modelo clássico:

```bash
npx create-docusaurus@latest manual classic --typescript
```

Depois, configure `title`, `url` e `baseUrl`, use `pt-BR` como idioma, mantenha
`onBrokenLinks: 'throw'`, retire as páginas e o blog de exemplo e escreva o
conteúdo da mudança atual.

Não crie Dockerfile, pipeline, publicação nem deploy do manual sem aprovação
explícita: o `AGENTS.md` protege essas configurações.

### Verificar

Rode `cd manual && npm run build` e informe o resultado no parágrafo `Testado:`
do commit. O build falha se houver link quebrado.

## 3. Confira antes de commitar

- [ ] Pendências do `HISTORICO.md` reconciliadas com `origin/main`.
- [ ] Uma linha nova descreve esta tarefa.
- [ ] Manual atualizado e build aprovado, ou mudança sem efeito para o usuário
      final.
- [ ] Arquivos do manual e do histórico incluídos no mesmo commit da tarefa.

Ao concluir, informe se o manual mudou e quais linhas do histórico continuam
`pendente`.
