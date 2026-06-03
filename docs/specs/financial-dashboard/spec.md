# Feature Specification: Financial Dashboard

**Feature**: `financial-dashboard`
**Created**: 2026-06-03
**Status**: Clarified
**Gerado por**: agente-00c, onda-003 (modo autonomo)
**Clarificado por**: agente-00c, onda-004 (clarify session 2026-06-03; dec-018..dec-021)
**Predecessor**: Constitution v1.0.0 (`docs/constitution.md`); Briefing (`docs/01-briefing-discovery/briefing.md`)

> Todos os itens [A VALIDAR] foram resolvidos em clarify (onda-004). Decisoes dec-018 a dec-021
> registram as resolucoes com score e justificativa. Principios da constitution (P-I a P-V)
> sao NON-NEGOTIABLE e ja estao refletidos nesta spec.

---

## User Scenarios & Testing

### User Story 1 - Gestor cadastra e gerencia vendedores (Priority: P1)

Como Gestor/Admin, quero cadastrar vendedores com seus percentuais de comissao individuais,
e poder atualizar, desativar ou remover vendedores, de modo que eu tenha controle total
sobre quem esta ativo no sistema e quais regras de comissao se aplicam a cada um.

**Why this priority**: Fundacao do sistema — sem vendedores cadastrados, nenhuma outra
funcionalidade opera. E o prerequisito de todas as demais stories.

**Independent Test**: Criar um vendedor, atualizar seu percentual de comissao, desativa-lo
e verificar que nao aparece mais como opcao ativa em novos pedidos — sem depender de
pedidos ou comissoes existentes.

**Acceptance Scenarios**:

1. **Given** um Gestor autenticado sem nenhum vendedor cadastrado, **When** ele preenche
   nome, email e percentual de comissao (ex: 5,5%) e confirma o cadastro, **Then** o
   vendedor aparece na lista de vendedores ativos com os dados informados.
2. **Given** um vendedor ativo com percentual de comissao de 5%, **When** o Gestor altera
   o percentual para 7%, **Then** a mudanca e registrada com versao nova da regra e a regra
   anterior permanece associada as comissoes ja apuradas (auditabilidade P-I).
3. **Given** um vendedor ativo, **When** o Gestor o desativa, **Then** o vendedor nao aparece
   como opcao em novos pedidos, mas seus historicos de vendas e comissoes permanecem acessiveis.
4. **Given** um Vendedor autenticado, **When** ele tenta acessar o cadastro de outro vendedor,
   **Then** o sistema nega o acesso (RBAC deny-by-default, P-IV).

---

### User Story 2 - Gestor registra pedidos de venda (Priority: P2)

Como Gestor/Admin, quero registrar pedidos associados a vendedores, com valor, data e status
inicial, e poder atualizar o status do pedido ao longo do seu ciclo de vida (rascunho →
confirmado → pago → cancelado), de modo que o sistema tenha o registro completo e auditavel
de todas as vendas realizadas.

**Why this priority**: Pedidos sao a materia-prima do calculo de comissao (P-II). Sem pedidos
registrados e status corretamente mantidos, nao ha como calcular comissoes.

**Independent Test**: Criar um pedido para um vendedor existente, transicionar o status de
`rascunho` ate `pago` e verificar que cada transicao esta registrada — sem depender do
calculo de comissao.

**Acceptance Scenarios**:

1. **Given** um Gestor autenticado e um vendedor ativo, **When** ele registra um pedido com
   valor R$ 1.500,00, data e vendedor associado, **Then** o pedido e criado com status
   `rascunho` e o valor e armazenado com precisao decimal (sem float, P-III).
2. **Given** um pedido com status `confirmado`, **When** o Gestor marca como `pago`,
   **Then** o status muda para `pago` e a data de pagamento e registrada.
3. **Given** um pedido com status `pago`, **When** se tenta reverter para `rascunho`,
   **Then** o sistema rejeita a transicao como invalida (maquina de estado explicita).
4. **Given** um pedido com status `pago`, **When** o Gestor cancela o pedido,
   **Then** o sistema registra o cancelamento (transicao `pago` → `cancelado` com trilha P-I)
   e, para cada comissao ja apurada referenciando esse pedido, cria automaticamente um
   Estorno de Comissao (valor negativo, referenciando a comissao original e o pedido
   cancelado). Se a comissao estiver `pendente`, o Estorno e aplicado diretamente. Se
   `aprovada` ou `paga`, o Estorno fica com status `pendente_aprovacao` ate o Financeiro
   confirmar o lancamento (FR-027).

---

### User Story 3 - Sistema apura comissoes mensalmente (Priority: P3)

Como Gestor/Admin, quero que o sistema apure automaticamente as comissoes de todos os
vendedores ao final de cada mes (ou sob demanda), calculando sobre os pedidos com status
`pago` no periodo, de modo que eu tenha a folha de comissoes precisa, auditavel e pronta
para aprovacao.

**Why this priority**: Core financeiro do sistema. Depende de P1 (vendedores) e P2 (pedidos),
mas e a proposta de valor central — substituir a planilha manual.

**Independent Test**: Para um vendedor com percentual de 10% e tres pedidos pagos de
R$ 1.000,00 cada no periodo, verificar que a comissao apurada e exatamente R$ 300,00,
referenciar cada pedido de origem, e que reexecutar a apuracao do mesmo periodo NAO
duplica a comissao (idempotencia, P-II).

**Acceptance Scenarios**:

1. **Given** um vendedor com percentual de comissao de 8% e dois pedidos pagos no mes de
   R$ 2.000,00 e R$ 3.000,00, **When** o sistema apura o periodo mensal, **Then** a comissao
   calculada e R$ 400,00 (8% de R$ 5.000,00), referenciando os dois pedidos de origem e o
   percentual vigente.
2. **Given** uma comissao ja apurada para o periodo, **When** o sistema apura o mesmo periodo
   novamente, **Then** nenhuma comissao duplicada e gerada (idempotencia P-II).
3. **Given** um pedido com status `confirmado` (nao `pago`), **When** o sistema apura o periodo,
   **Then** esse pedido NAO entra no calculo de comissao (P-II: comissao so sobre pedido `pago`).
4. **Given** o Gestor alterou o percentual de comissao de um vendedor no meio do mes, **When**
   o sistema apura o periodo, **Then** as comissoes apuradas referenciam a versao da Regra de
   Comissao vigente na DATA DO PEDIDO (nao da apuracao) — pedidos anteriores a mudanca usam
   o percentual anterior; pedidos posteriores usam o novo (confirmado em clarify, dec-020).

---

### User Story 4 - Financeiro controla pagamentos de comissoes (Priority: P4)

Como Financeiro, quero visualizar as comissoes apuradas e pendentes de pagamento, aprova-las
e registrar o pagamento efetivo, com trilha de cada acao realizada, de modo que haja controle
formal e auditavel do fluxo de desembolso de comissoes.

**Why this priority**: Completa o ciclo financeiro. Sem esta story, as comissoes sao calculadas
mas nunca efetivadas. Dependente de P3.

**Independent Test**: Para uma comissao apurada com status `pendente`, o Financeiro pode
aprova-la (transicao para `aprovado`) e marcar como paga (transicao para `pago`), com cada
transicao registrada com ator e timestamp — sem depender de dashboards ou relatorios.

**Acceptance Scenarios**:

1. **Given** uma comissao com status `pendente`, **When** o Financeiro a aprova, **Then**
   o status muda para `aprovado` e um registro de trilha e criado com: ator (Financeiro),
   timestamp UTC, acao realizada (P-I).
2. **Given** uma comissao `aprovada`, **When** o Financeiro registra o pagamento efetivo,
   **Then** o status muda para `pago` e o registro de trilha e atualizado com ator, timestamp
   e valor pago.
3. **Given** um Vendedor autenticado, **When** ele tenta aprovar ou marcar como paga uma
   comissao, **Then** o sistema nega a operacao (RBAC P-IV: Vendedor nao tem permissao sobre
   pagamentos).
4. **Given** um Gestor autenticado, **When** ele acessa a lista de comissoes, **Then** ele ve
   TODAS as comissoes de todos os vendedores; **When** um Vendedor acessa, **Then** ele ve
   SOMENTE as suas proprias.

---

### User Story 5 - Dashboards de performance de vendas (Priority: P5)

Como Gestor/Admin ou Vendedor, quero visualizar metricas de performance de vendas em
dashboards interativos, com filtros por periodo e (para o Gestor) por vendedor, de modo
que eu possa acompanhar resultados de forma rapida e confiavel.

**Why this priority**: Produto visivel para o usuario final, mas derivado dos dados ja
gerados pelas stories anteriores. Entrega valor de UX apos o motor financeiro estar
correto.

**Independent Test**: Para dados de vendas conhecidos (pedidos e valores fixos), verificar
que o dashboard exibe os totais corretos derivados deterministicamente dos registros
transacionais — sem valores inventados ou cacheados de forma incorreta (P-I).

**Acceptance Scenarios**:

1. **Given** um Gestor autenticado, **When** ele acessa o dashboard consolidado do mes corrente,
   **Then** ele ve: total de vendas (soma dos pedidos pagos), total de comissoes apuradas e
   ranking dos vendedores por volume de vendas.
2. **Given** um Gestor autenticado, **When** ele filtra por um vendedor especifico e um periodo,
   **Then** o dashboard exibe apenas os dados daquele vendedor no periodo selecionado.
3. **Given** um Vendedor autenticado, **When** ele acessa o dashboard, **Then** ele ve SOMENTE
   suas proprias metricas (volume de vendas e comissoes proprias) — sem dados de outros
   vendedores (RBAC P-IV).
4. **Given** um valor exibido no dashboard, **When** o Gestor inspeciona o detalhe,
   **Then** e possivel rastrear o valor ate os pedidos individuais que o compoem (P-I:
   nenhum valor consolidado sem rastreabilidade).

---

### Edge Cases

- O que acontece se o percentual de comissao de um vendedor e zero? O sistema deve
  permitir (comissao zero e valida) e registrar a apuracao normalmente.
- Como o sistema trata dois pedidos com o mesmo valor no mesmo dia para o mesmo vendedor?
  Devem ser registrados como pedidos distintos com IDs unicos.
- O que acontece se a apuracao mensal falha a meio (ex: crash durante o processamento)?
  A reexecucao deve ser idempotente e nao duplicar comissoes ja persistidas (P-II).
- O que acontece se um pedido e cancelado apos a comissao ja ter sido apurada e aprovada?
  Cria-se um Estorno de Comissao (entidade separada, valor negativo) referenciando a comissao
  original; os registros imutaveis nao sao alterados (P-I). O Estorno fica pendente de
  aprovacao do Financeiro se a comissao ja estava aprovada ou paga (FR-027, dec-021).
- O que acontece se nenhum pedido foi pago no periodo? A apuracao retorna resultado vazio
  sem erro.
- Como lidar com datas em fuso horario diferente? Todas as operacoes usam UTC como
  referencia interna e de persistencia; exibicao local pode ser ajustada no frontend.
  Multi-fuso nao e requisito do MVP — UTF unico e suficiente para POC (padrão de mercado
  para sistemas financeiros de escopo nacional).

---

## Requirements

### Functional Requirements

#### Dominio 1: Gestao de Vendedores

- **FR-001**: O sistema MUST permitir ao Gestor/Admin criar, editar, visualizar e desativar
  vendedores (CRUD completo).
- **FR-002**: Cada vendedor MUST ter um percentual de comissao individual configuravel,
  expresso como valor decimal com precisao suficiente para evitar erro de arredondamento
  (P-III). O intervalo valido e [0%, 100%] — comissao zero e valida; percentual negativo
  ou acima de 100% MUST ser rejeitado com mensagem de erro explicita.
- **FR-003**: Toda alteracao de percentual de comissao MUST criar uma nova versao da regra,
  preservando o historico de versoes anteriores para auditoria (P-I, P-II).
- **FR-004**: O sistema MUST manter dados pessoais de vendedores limitados ao minimo
  necessario: nome, email, identificador unico e percentual de comissao. Campos adicionais
  MUST ser justificados por finalidade explicita (P-V).
- **FR-005**: O sistema MUST suportar exclusao de dados pessoais de vendedor sob demanda do
  titular (direito LGPD), preservando os registros financeiros anonimizados necessarios para
  auditoria (P-I prevalece sobre exclusao irrestrita; politica a detalhar em plan).

#### Dominio 2: Registro de Pedidos

- **FR-006**: O sistema MUST permitir ao Gestor/Admin criar pedidos com: vendedor associado,
  valor total, data do pedido e status inicial `rascunho`.
- **FR-007**: O valor de cada pedido MUST ser armazenado e processado sem ponto flutuante
  binario; representacao MUST usar precisao decimal adequada (P-III).
- **FR-008**: O sistema MUST implementar a maquina de estado de pedido com as seguintes
  transicoes validas (confirmado em clarify, dec-019):
  - `rascunho` → `confirmado` (Gestor confirma o pedido)
  - `confirmado` → `pago` (Gestor registra pagamento do pedido pelo cliente)
  - `confirmado` → `cancelado` (Gestor cancela antes do pagamento)
  - `pago` → `cancelado` (Gestor cancela pos-pagamento — aciona criacao de Estorno de Comissao se houver comissao apurada; ver FR-027)
  - Todas as demais transicoes MUST ser rejeitadas pelo sistema.
- **FR-009**: Toda transicao de estado de pedido MUST registrar: ator, timestamp UTC e
  estado anterior/novo (trilha append-only, P-I).
- **FR-010**: O sistema MUST permitir associar itens de linha ao pedido (produto/descricao e
  valor unitario) para fins de rastreabilidade; o valor total do pedido MUST ser consistente
  com a soma dos itens quando itens forem informados.

#### Dominio 3: Calculo de Comissao

- **FR-011**: O motor de comissao MUST calcular comissao SOMENTE sobre pedidos com status
  `pago`; pedidos em quaisquer outros status MUST NOT gerar comissao (P-II).
- **FR-012**: A formula de calculo MUST ser: `comissao = valor_total_pedido_pago * percentual_vigente`,
  onde `percentual_vigente` e o percentual do vendedor vigente na DATA DO PEDIDO (`data_do_pedido`)
  — a versao da Regra de Comissao ativa nessa data e selecionada independentemente de quando
  ocorre a apuracao (confirmado em clarify, dec-020).
- **FR-013**: Cada comissao apurada MUST referenciar de forma imutavel: o ID do pedido de
  origem, o ID do vendedor, o percentual aplicado, a versao da regra de comissao e o periodo
  de apuracao (P-I, P-II).
- **FR-014**: O sistema MUST ser idempotente na apuracao: reprocessar o mesmo periodo MUST NOT
  criar comissoes duplicadas nem alterar comissoes ja persistidas (P-II).
- **FR-015**: A apuracao de comissoes MUST ser executavel por periodo mensal, sob demanda pelo
  Gestor/Admin; apuracao automatica periodica e desejavel pos-MVP.

> Decisoes de infraestrutura: apuracao e operacao sob demanda no MVP (FR-015); sem scheduler
> automatico no MVP. Agendamento automatico e pos-MVP. Sem criptografia de dados em repouso
> alem do que o PostgreSQL oferece nativamente no MVP (sem key rotation).

#### Dominio 4: Dashboards de Performance

- **FR-016**: O sistema MUST exibir para o Gestor/Admin um dashboard consolidado com:
  total de vendas (soma dos pedidos pagos), total de comissoes apuradas e lista de vendedores
  com seus respectivos volumes, filtraveis por periodo (mes/ano ou intervalo de datas).
- **FR-017**: O sistema MUST exibir para o Vendedor um dashboard individual com: suas proprias
  vendas, suas proprias comissoes apuradas e status de pagamentos — sem expor dados de outros
  vendedores (P-IV).
- **FR-018**: Todo valor exibido em dashboard MUST ser derivado deterministicamente dos
  registros transacionais subjacentes; valores pre-calculados ou cacheados MUST ser
  reconcomputaveis sob demanda (P-I).
- **FR-019**: O sistema MUST exibir indicadores de status de comissoes pendentes de aprovacao
  e pagamento para o Gestor/Admin e Financeiro.

#### Dominio 3.bis: Estorno de Comissao (cancelamento pos-apuracao)

- **FR-027**: Quando um pedido `pago` e cancelado (transicao `pago` → `cancelado`) e existir
  Comissao ja apurada referenciando esse pedido, o sistema MUST criar automaticamente um
  registro de Estorno de Comissao com: valor negativo igual ao valor da comissao original,
  referencia imutavel a comissao de origem e ao pedido cancelado, e timestamp UTC (P-I, P-II).
- **FR-028**: Se a comissao de origem estiver com status `pendente`, o Estorno MUST ser
  aplicado automaticamente, zerando o valor liquido a pagar. Se estiver com status `aprovado`
  ou `pago`, o Estorno MUST ficar com status `pendente_aprovacao` ate o Financeiro confirmar
  o lancamento (mesmo fluxo de aprovacao do FR-020).
- **FR-029**: Os registros originais de Comissao MUST NOT ser alterados ou removidos em
  nenhum cenario de cancelamento; a imutabilidade dos registros financeiros e absoluta (P-I).
  O saldo liquido de uma comissao e derivado computacionalmente (comissao + estornos
  associados), nao armazenado como campo editavel.

#### Dominio 5: Controle de Pagamentos de Comissao

- **FR-020**: O sistema MUST implementar a maquina de estado de pagamento de comissao com as
  seguintes transicoes validas:
  - `pendente` → `aprovado` (Financeiro aprova a comissao para pagamento)
  - `aprovado` → `pago` (Financeiro registra o pagamento efetivo)
  - `aprovado` → `pendente` (Financeiro reverte aprovacao — com motivo obrigatorio)
  - Todas as demais transicoes MUST ser rejeitadas.
- **FR-021**: Cada transicao de estado de pagamento de comissao MUST ser registrada em trilha
  append-only com: ator, timestamp UTC, estado anterior, estado novo e motivo (obrigatorio em
  reversoes) (P-I).
- **FR-022**: O Financeiro MUST visualizar a lista de comissoes por status (pendentes,
  aprovadas, pagas), com possibilidade de filtrar por periodo e vendedor.
- **FR-023**: O sistema MUST impedir que o Vendedor execute qualquer operacao de aprovacao ou
  registro de pagamento; o Financeiro MUST NOT poder alterar regras de comissao ou cadastros
  de vendedores (RBAC P-IV, deny-by-default).

#### Dominio 6: Autenticacao e Autorizacao

- **FR-024**: O sistema MUST autenticar usuarios antes de qualquer acesso. O mecanismo
  concreto (JWT stateless vs sessao server-side) sera definido em plan — ambas as opcoes
  sao compatíveis com os Core Principles; a escolha e decisao tecnica da etapa plan.
- **FR-025**: O sistema MUST aplicar controle de acesso baseado em papel (RBAC) em todas as
  operacoes no backend (server-side); controles de UI sao complementares, nao barreiras de
  seguranca (P-IV).
- **FR-026**: O sistema MUST negar por padrao qualquer acesso nao explicitamente autorizado
  pelo papel do usuario autenticado (deny-by-default, P-IV).

### Key Entities

- **Vendedor**: representa um membro da equipe de vendas com papel no sistema. Atributos:
  identificador unico, nome, email, status (ativo/inativo), historico de percentuais de comissao
  (versionado), data de cadastro. Dado pessoal — sujeito a LGPD (P-V).

- **Regra de Comissao**: versao especifica do percentual de comissao de um vendedor. Atributos:
  ID do vendedor, percentual (decimal sem float), data de vigencia inicio, data de vigencia fim
  (nulo se vigente), versao sequencial. Imutavel apos criacao.

- **Pedido**: registro de uma venda. Atributos: identificador unico, ID do vendedor, valor total
  (decimal), data do pedido, status (maquina de estado), itens de linha (lista opcional),
  trilha de transicoes de estado.

- **Item de Pedido**: linha de detalhe de um pedido. Atributos: descricao/produto, quantidade,
  valor unitario (decimal), valor total da linha.

- **Comissao**: resultado da apuracao de comissao sobre um pedido pago. Atributos: ID unico,
  ID do pedido de origem, ID do vendedor, valor calculado (decimal), percentual aplicado, ID da
  versao de regra aplicada, periodo de apuracao (mes/ano), status de pagamento, data de apuracao.
  Imutavel apos criacao.

- **Pagamento de Comissao**: registro do ciclo de pagamento de uma ou mais comissoes. Atributos:
  ID unico, lista de IDs de comissoes, status (pendente/aprovado/pago), trilha de transicoes
  (append-only), ator de cada transicao, timestamps UTC.

- **Estorno de Comissao**: registro de cancelamento de comissao ja apurada (criado quando
  pedido `pago` e cancelado pos-apuracao). Atributos: ID unico, ID da comissao de origem,
  ID do pedido cancelado, valor negativo (igual ao valor da comissao original), status
  (`aplicado` | `pendente_aprovacao` | `aprovado` | `lancado`), timestamp UTC de criacao,
  ator que triggerou (via cancelamento do pedido). Imutavel apos criacao (P-I).

- **Trilha de Auditoria**: registro append-only de eventos criticos (transicoes de estado de
  pedido, transicoes de pagamento de comissao, alteracoes de regra de comissao). Imutavel por
  definicao (P-I).

- **Usuario**: representa um ator autenticado no sistema. Atributos: identificador, nome, email,
  papel (Gestor/Admin | Vendedor | Financeiro). Um Vendedor-usuario MUST estar vinculado a uma
  entidade Vendedor para restricoes de escopo de dados.

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: O Gestor consegue cadastrar um vendedor, registrar um pedido e acionar a apuracao
  de comissao em menos de 5 minutos do zero — sem precisar de instrucoes externas.
- **SC-002**: Dado um conjunto de pedidos pagos com valores e percentuais conhecidos, o sistema
  calcula a comissao com resultado identico em 100% das execucoes (determinismo, P-II).
- **SC-003**: Reprocessar a apuracao do mesmo periodo duas vezes produz exatamente o mesmo
  numero de registros de comissao — sem duplicatas (idempotencia, P-II).
- **SC-004**: Qualquer valor exibido em dashboard pode ser rastreado ate os pedidos individuais
  de origem em no maximo 3 cliques (auditabilidade, P-I).
- **SC-005**: Uma requisicao de Vendedor tentando acessar dados de outro vendedor e negada em
  100% dos casos em todos os endpoints (RBAC, P-IV).
- **SC-006**: Toda transicao de estado de pagamento de comissao tem trilha com ator, timestamp
  e estado anterior/novo — sem excecao (auditabilidade, P-I).
- **SC-007**: O sistema nao persiste nenhum valor monetario como ponto flutuante; auditoria do
  schema de banco confirma ausencia de colunas float em campos monetarios (P-III).
- **SC-008**: O inventario de dados pessoais de vendedores mapeia cada campo coletado a uma
  finalidade explicita do MVP (LGPD, P-V).
- **SC-009**: O dashboard do Gestor carrega os dados do mes corrente em menos de 3 segundos
  para bases com ate 10.000 pedidos.

---

## Clarifications

### Session 2026-06-03

- Q: Os 3 atores inferidos (Gestor/Admin, Vendedor, Financeiro) e suas fronteiras de acesso sao corretos? → A: Confirmado. Gestor/Admin: administracao geral (vendedores, pedidos, percentuais, todos os dashboards e comissoes). Vendedor: somente proprias metricas e proprias comissoes. Financeiro: aprovar e registrar pagamentos de comissoes; sem acesso a regras de comissao ou cadastros. (dec-018)
- Q: A maquina de estado de pedido (rascunho→confirmado→pago; confirmado→cancelado; pago→cancelado) esta completa? → A: Confirmado. 4 estados, 4 transicoes validas; todas as demais sao rejeitadas pelo sistema. Transicao pago→cancelado e valida e aciona politica de estorno de comissao. (dec-019)
- Q: O percentual de comissao vigente e selecionado pela DATA DO PEDIDO (nao pela data de apuracao)? → A: Confirmado. A versao da Regra de Comissao vigente na data do pedido (campo `data_do_pedido`) e o criterio canônico para selecao. Apurar no mes seguinte nao muda a versao de regra aplicada. (dec-020)
- Q: Qual a politica de cancelamento de pedido quando a comissao ja foi apurada (e possivelmente aprovada)? → A: Adotar padrao conservador de mercado alinhado a P-I e P-II: criar registro de Estorno de Comissao (entidade separada, valor negativo, referenciando a comissao original e o pedido cancelado) sem modificar nem remover os registros imutaveis existentes. Se a comissao ja estiver aprovada ou paga, o Estorno fica pendente de aprovacao pelo Financeiro antes do lancamento. (dec-021)

---

## Resolved Ambiguities

> Itens resolvidos — A-001, A-004, A-005 confirmados em clarify (session 2026-06-03); A-006 resolvido com padrao conservador de mercado + P-I/P-II.

| ID | Ambiguidade | Resolucao adotada | Confianca | Status |
|----|-------------|-------------------|-----------|--------|
| A-001 | 3 atores (Gestor, Vendedor, Financeiro) | Confirmados (dec-018). Fronteiras: Gestor=admin total; Vendedor=proprias metricas/comissoes; Financeiro=pagamentos | Alta | Resolvido |
| A-002 | Regra de comissao: %-sobre-pedido-pago | Percentual por vendedor sobre valor total do pedido pago (dec-004) | Alta | Resolvido |
| A-003 | Periodicidade de apuracao | Mensal, sob demanda no MVP (dec-005) | Alta | Resolvido |
| A-004 | Estados de pedido | rascunho/confirmado/pago/cancelado — 4 transicoes validas confirmadas (dec-019) | Alta | Resolvido |
| A-005 | Criterio de data para versao de regra | Percentual vigente na DATA DO PEDIDO (nao da apuracao) — confirmado (dec-020) | Alta | Resolvido |
| A-006 | Politica de cancelamento pos-comissao | Registro de Estorno de Comissao (valor negativo, entidade separada, ref imutavel a comissao original; aprovacao Financeiro se comissao ja aprovada/paga) — padrao conservador P-I/P-II (dec-021) | Alta | Resolvido |

---

## Fora de Escopo (MVP)

- Integracao com ERP / sistemas de contabilidade externa.
- Emissao fiscal / NF-e.
- Multi-moeda.
- Multi-tenant.
- Metas de venda por vendedor/periodo.
- Comissao por faixa de valor e/ou por produto.
- Exportacao de relatorios (CSV, PDF).
- Notificacoes (email, push) de eventos de comissao/pagamento.
- Agendamento automatico de apuracao mensal (apuracao e sob demanda no MVP).
- Integracao com gateway de pagamento (nao ha PCI-DSS no MVP).
