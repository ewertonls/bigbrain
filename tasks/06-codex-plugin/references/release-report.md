# BB-0034 · Entrega do plugin local do Codex

O BigBrain recupera decisões com suas fontes ao iniciar, retomar e compactar uma sessão do Codex. O usuário pode revisar um handoff, editar ou descartar o rascunho e salvar a decisão e sua evidência no projeto. O daemon Go continua responsável pelas gravações no LadybugDB. [Guia de instalação, atualização e remoção](../../../integrations/README.md#install-the-local-codex-plugin)

## Entregas por task

| Task | Entrega | Commit |
| --- | --- | --- |
| BB-0035 | Descoberta, autorização e ciclo de vida dos hooks no cliente real | `8a77072` |
| BB-0036 | Pacote macOS ARM64 relocável com biblioteca nativa e licenças | `9619ab2` |
| BB-0037 | Marketplace do repositório, pacote e launcher | `1d9fdb8` |
| BB-0038 | Vínculo por projeto, worktree, sessão e agente, com diagnóstico | `4997700` |
| BB-0039 | Skills de recall, checkpoint e exploração | `527b330` |
| BB-0040 | Contexto com referências, orçamento, prévia e recibo | `9eebb64` |
| BB-0041 | Recall no início, na retomada e após compactação | `7353a15` |
| BB-0042 | Rascunhos privados com edição e descarte por revisão | `1eb38dc` |
| BB-0043 | Gravação revisada com retomada após perda de confirmação | `cd32bc7` |
| BB-0044 | Interface servida pelo Go em loopback com autenticação por execução | `5646fda` |
| BB-0045 | Explorer com caminho de recuperação, histórico e inspeção da fonte | `3e3d66b` |
| BB-0046 | Revisão de conteúdo, fonte, destino e operações antes de salvar | `1e9ec65` |
| BB-0047 | Automação com recibo, orçamento, prévia e pausa por sessão | `3246708` |
| BB-0048 | Teste de capacidade MCP Apps e registro do resultado. Teste inicial no app-server | `8ae01b7` |
| BB-0049 | Instalação manual, atualização e remoção | `b964bf2` |
| BB-0050 | Aceitação de ponta a ponta e regressão de replay e continuidade | `5392dec` |

## Verificação da entrega

- **Sessão real.** Em macOS ARM64 27.2.1, com Codex CLI/app-server 0.172.0 e LadybugDB 0.17.0, uma sessão nova recebeu um handoff salvo em outra sessão. Reconectamos o cliente, retomamos a sessão e solicitamos compactação. Na primeira resposta seguinte, o modelo devolveu o token e a fonte da evidência sem ferramentas. O recibo da compactação continha o mesmo ID e registrou 307 bytes de um limite de 8.192. A chamada ao modelo usou rede. Os hooks instalados também passaram no teste local independente. [Registro de aceitação](../15-release-verification.md)
- **Integridade.** O teste permanente simula uma gravação aceita pelo daemon cuja confirmação ainda não chegou ao cliente. A repetição recupera o ID original, conserva o vínculo e permite recuperar a fonte em outra sessão. Uma proposta descartada permanece fora da busca. [Teste de integração](../../../tests/acceptance/plugin.go)
- **Interface.** O Chromium acessou somente o loopback. A busca por teclado levou à decisão, ao relacionamento e à fonte. A revisão salvou um rascunho no projeto selecionado. A prévia e a pausa funcionaram em 320 px de largura. [Explorer](../images/verified-explorer.png) · [Session review](../images/verified-review.png) · [Automation](../images/verified-automation.png)
- **Distribuição e escopo.** Verificamos instalação fora do checkout, atualização por Git, troca do binário, desativação e remoção. Desativar o plugin retirou os três hooks do cliente. Remover o pacote preservou a memória e o rascunho. A suíte cobriu dois projetos, escopo global e dois worktrees. Sem embeddings, a busca retornou resultados por texto e relações e informou o motivo do fallback.
- **Verificações executadas.** Passaram `make test`, os cinco cenários de `make test-integration`, `go vet`, o detector de corrida do pacote CLI, a compilação do pacote relocável e a validação do Kadai.

## Ajustes feitos durante a revisão

- A identidade da base no plugin passou a resolver o prefixo físico de diretórios antes da primeira gravação. Isso mantém os rascunhos acessíveis em caminhos com symlink e preserva o contrato existente de `db-path`.
- O salvamento só passa para concluído depois de publicar os IDs usados pela próxima sessão. Chaves e conteúdo ficam congelados durante o replay. O recall conserva toda a seleção mesmo quando só uma memória mudou e aplica o prazo de dois segundos à operação inteira.
- O launcher passou a aceitar um ponteiro privado compartilhado para que o shell e o plugin encontrem o mesmo binário. Em um shell Codex restrito, o acesso ao socket Unix da base precisou de uma permissão específica para recuperar a evidência.
- O cliente testado leu o recurso HTML de MCP Apps, mas o teste não demonstrou renderização nem uma chamada originada pela interface embutida. A entrega usa o navegador local. Na validação posterior da BB-0048, o Codex Desktop abriu o Explorer ao lado do chat e exibiu a fonte da memória selecionada. O perfil de distribuição verificado é macOS ARM64. [Teste da extensão opcional](../13-embedded-viewer.md)
