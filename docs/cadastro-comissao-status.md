# Cadastro de comissão — estado salvo

- Branch: `cadastro-comissão`, criada a partir da `main` local.
- Commit base: `f99480ca650e7686523784e0999e534e4427f7d7`.
- Solicitação: configurar comissão individual por profissional e serviço, com alternância entre porcentagem e valor fixo.
- Implementação: menu Comissão nos detalhes do profissional, componente de edição, tipos e chamadas de API, endpoints GET/PUT protegidos para gestores, repositório e migração `021_professional_commissions.sql`.
- Unidade armazenada: centésimos de porcentagem para `percentage`; centavos para `fixed`.
- Percentual permitido: 0 a 100; valor fixo não negativo. Campo vazio remove a configuração ao salvar.
- Validações executadas: build do frontend aprovado; 6 testes existentes de ProfessionalsManager aprovados; `go test ./...` aprovado; `git diff --check` aprovado.
- Migração criada, ainda não aplicada ao banco. Nenhum deploy ou push realizado.
- Testes específicos da nova tela e persistência das comissões ainda não foram adicionados. O fake de testes apenas retorna os valores recebidos.
- Trabalho interrompido a pedido do usuário e posteriormente salvo com este registro. A pasta não rastreada `.e2e-artifacts/` foi preservada fora do commit.
