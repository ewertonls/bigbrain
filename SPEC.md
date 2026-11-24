# BigBrain — especificação técnica inicial

*Proposta para revisão.*

Quando um agente retoma um projeto, pode encontrar a decisão anterior e ainda precisar descobrir por que ela foi tomada. O objetivo do BigBrain é preservar memórias e as relações entre elas ao longo de várias sessões. Um segundo agente deve conseguir chegar da decisão à evidência que a motivou, mesmo quando os dois registros usam palavras diferentes.

| Estado | Orientação |
| --- | --- |
| Premissa de projeto | Partir de LadybugDB para a primeira avaliação. SQLite foi considerado como alternativa. A adequação do LadybugDB a esta carga ainda precisa ser medida. |
| Experiência pretendida | Avaliar CLI e daemon no mesmo executável. Um comando de dados inicia o daemon em segundo plano quando ele está ausente e reutiliza a instância ativa nas chamadas seguintes. O usuário também pode encerrá-lo pela CLI. |
| Proposta de MVP | Memória local, independente do agente ou modelo, com escopo de projeto por padrão e conhecimento global escolhido explicitamente. A primeira avaliação usa texto e relações registradas, com a rede desligada. |
| Decisões em aberto | Linguagem, primeiro sistema operacional e arquitetura de CPU, transporte local, sintaxe da CLI, esquema físico, algoritmo de ordenação e eventual uso de embeddings. Go e consultas Cypher são candidatos para o protótipo. |

## O que guardar e quem pode consultar

Uma **memória** tem um ID estável, um escopo, texto, uma referência de origem e o instante de registro. Pode representar uma decisão, observação ou hipótese. Uma **relação** liga duas memórias do mesmo escopo e registra direção, tipo, peso e sua própria proveniência. Assim, quem consulta consegue distinguir a fonte de uma observação de quem propôs ligá-la a uma decisão. O peso expressa a prioridade do vínculo na recuperação. A veracidade da afirmação depende da fonte.

Na CLI proposta, o agente seleciona o projeto por um identificador estável. Se faltar o projeto, a operação pede que o cliente indique o escopo. **Global** designa uma coleção local da mesma conta. Para compartilhar uma lição, o usuário revisa o texto e a referência que podem circular e grava uma nova memória global. O original e suas relações continuam no projeto. Cada consulta de projeto pode incluir a coleção global por escolha explícita.

O daemon aplica o escopo antes da busca e em cada travessia, inclusive nas consultas por ID e nas explicações. Conteúdo, contagens e referências de origem obedecem à mesma regra. O isolamento entre projetos protege o contexto de trabalho. A conta do sistema operacional é a fronteira de acesso aos arquivos e ao canal local.

**Memória recuperada é evidência.** A resposta entrega o texto em um campo de dados, com sua origem e o motivo da recuperação. Mesmo que o texto contenha uma ordem, a consulta é somente leitura. Qualquer ação posterior de um agente requer autorização própria. Afirmações conflitantes conservam suas fontes e podem ser examinadas lado a lado.

## Como provar o valor das conexões

O MVP precisa de três operações de dados pela CLI. A primeira registra uma memória e devolve seu ID. A segunda liga dois IDs existentes e registra a origem do vínculo. A terceira consulta o escopo selecionado e devolve até o limite de resultados solicitado, com IDs e fontes em formato legível por agentes.

Uma consulta textual encontra as memórias que contêm os termos pesquisados. No modo associativo, a mesma busca fornece os pontos de partida. O daemon percorre um salto por relações explícitas, limita os vizinhos examinados por memória e usa o peso para priorizá-los. Reúne os resultados por ID, preserva o melhor acerto textual e informa como chegou a cada vizinho. O primeiro ensaio pode usar uma varredura de texto e desempate estável por ID.

### Ensaio de retomada

Um agente registra os seguintes dados fictícios no projeto A e encerra a sessão.

| ID | Registro | Origem |
| --- | --- | --- |
| D1 | Órbita envia as leituras em lote. | Decisão 12 |
| E1 | O receptor aceita apenas uma requisição por minuto. | Ensaio 7 |
| E2 | O painel usa fundo claro. | Nota 4 |

O agente liga D1 a E1 com a relação `justificada_por` e atribui o vínculo à revisão da Decisão 12. Um segundo agente consulta apenas `Órbita`, com limite de dois resultados.

- **Controle textual.** Recuperar D1. É o único registro cujo texto contém o termo buscado.
- **Com a relação.** Recuperar D1 e E1. A explicação de E1 mostra o caminho `D1 → justificada_por → E1`, a fonte do vínculo e o Ensaio 7. E2 fica de fora.
- **Controle sem a relação.** Repetir a consulta associativa numa cópia com os mesmos textos e sem o vínculo. Apenas D1 deve voltar. Reabrir a base original deve restaurar o resultado com D1 e E1.

Antes de ajustar a ordenação, separar seis perguntas de retomada, três de resposta textual direta e três que dependam de relações. Anotar de antemão qual memória sustenta cada resposta. Comparar os dois modos na mesma base com limite de três resultados. Contar um acerto quando a memória esperada e sua fonte aparecem entre os três primeiros. O critério proposto é ganhar pelo menos dois acertos entre as perguntas associativas e preservar os acertos diretos. Registrar também quantos resultados irrelevantes cada modo introduziu.

## CLI, concorrência e ciclo de vida

A CLI envia operações por um canal local restrito à conta. Um daemon do BigBrain por base física mantém o banco aberto e atende os clientes. A documentação do LadybugDB prevê um único objeto de banco em modo de leitura e escrita, com conexões derivadas dele. Esse é o motivo para concentrar o acesso ao arquivo no daemon. [Concorrência no LadybugDB](https://docs.ladybugdb.com/concurrency/)

| Situação | Comportamento proposto |
| --- | --- |
| Primeiro comando de dados | A CLI tenta o canal da base. Se a instância estiver ausente, um único cliente ganha a eleição de partida e inicia o próprio executável em modo daemon. Os demais aguardam. A instância só anuncia prontidão depois de obter a posse exclusiva da base e abri-la. A espera tem prazo e devolve diagnóstico em caso de falha. |
| Instância em execução | A CLI verifica a identidade da base e a versão do protocolo e reutiliza o processo. Incompatibilidade gera orientação para alinhar as versões. Ajuda e consulta de estado funcionam com o serviço parado. |
| Escritas simultâneas | O daemon pode enfileirar as escritas. Confirma cada operação após gravá-la junto com sua chave de repetição na mesma transação. Reenviar a mesma chave e os mesmos dados devolve o resultado anterior. Reutilizar a chave com dados diferentes gera conflito. |
| Parada solicitada | O daemon recusa novos pedidos, conclui os já aceitos, fecha a base e libera o canal e a posse exclusiva. A CLI confirma a parada ao observar o encerramento daquela instância. Clientes que chegam durante a parada recebem um estado temporário. Pedir a parada novamente com o serviço ausente termina com sucesso. |
| Queda ou canal órfão | Uma nova partida verifica o bloqueio do sistema e a identidade da instância antes de limpar o canal antigo. Com a posse exclusiva assegurada, reabre a base e verifica sua integridade. Uma base ocupada, ilegível ou incompatível produz diagnóstico e preserva os arquivos. O cliente pode repetir uma escrita de resultado incerto com a chave original. |

Um tempo configurável de inatividade é uma extensão dessa mesma parada. Só começa a contar quando a fila e as operações em curso estiverem vazias. O valor padrão fica para depois das medições de partida. O transporte também depende da plataforma escolhida, por exemplo, um socket de domínio Unix ou um pipe nomeado.

### Critérios operacionais de aceite

1. Iniciar duas CLIs ao mesmo tempo sobre uma base parada. Ambas terminam, são atendidas pela mesma instância e duas gravações distintas permanecem visíveis após encerrar e reiniciar o daemon.
2. Interromper o daemon depois do commit e antes da entrega da resposta. Repetir o pedido deve devolver o mesmo ID e deixar uma única memória. Repetir o ensaio com uma escrita nova e interrupção antes do commit. Ao reabrir a base, a memória e sua chave devem estar ambas ausentes.
3. Pedir a parada durante uma escrita aceita. A escrita termina, novos pedidos recebem o estado de parada e duas solicitações consecutivas de encerramento deixam a base fechada.
4. Criar no projeto B uma memória com o mesmo termo do ensaio e criar uma terceira no escopo global. Consultas de A por texto, vínculo e ID só podem revelar A. Ao incluir o global, a memória global pode aparecer e B continua inacessível. Uma tentativa de ligar A a B deve falhar.
5. Guardar como dado de teste um texto que peça para apagar uma memória. Consultá-lo deve apenas devolver o texto e a fonte. O conjunto de registros e relações deve ser idêntico antes e depois.

## Primeiros riscos a validar

| Questão | Evidência para decidir |
| --- | --- |
| O LadybugDB atende às escritas curtas e consultas de um salto? | Fixar a versão do motor e da biblioteca cliente. Em uma base descartável, intercalar registro, criação de vínculos e leitura com dois clientes. Medir latência de partida e consulta, conferir IDs e relações após queda e registrar os limites encontrados. |
| Como distribuir o executável? | A API Go do LadybugDB usa a API C. Para o primeiro par de sistema e CPU candidato, listar bibliotecas nativas e demais arquivos exigidos. Testar a instalação do pacote em uma máquina limpa, sem compilador, e executar o ensaio completo offline. Esse resultado orienta a escolha de linguagem e empacotamento. [Instalação oficial](https://docs.ladybugdb.com/installation/#go) |
| Como preservar memórias ao atualizar o motor? | Exportar uma base de teste e importar em uma nova, preservando a original. Comparar IDs, escopos, fontes, relações e resultados do ensaio antes de adotar uma atualização. [Migração no LadybugDB](https://docs.ladybugdb.com/migrate/) |

## Depois do MVP

Se perguntas relevantes ficarem sem um ponto de partida textual, comparar a busca atual com busca de texto indexada e com candidatos vetoriais. A documentação lista extensões para texto e vetores. A avaliação deve medir ganho nas mesmas perguntas, tempo de indexação e funcionamento offline antes de escolher embeddings ou um provedor de inferência intercambiável. [Extensões do LadybugDB](https://docs.ladybugdb.com/extensions/)

Um adaptador para o Model Context Protocol (MCP) poderá expor as operações da CLI com os mesmos limites de escopo. Consolidação de duplicatas, reconciliação de versões conflitantes e esquecimento por política exigem critérios próprios de proveniência e retenção. Sincronização remota e uso multiusuário também ficam para uma etapa posterior.
