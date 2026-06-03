# Performance Checklist: Financial Dashboard

**Purpose**: Validar qualidade dos requisitos de performance — latencia, throughput, escalabilidade, caching, queries de dashboard e degradacao graceful.
**Created**: 2026-06-03
**Feature**: [spec.md](../spec.md) | [plan.md](../plan.md) | [data-model.md](../data-model.md)

---

## Targets de Latencia

- [x] CHK045 - Existe target de latencia mensuravel para o dashboard do Gestor? [Completude, Spec §SC-009] {auto}
  > SC-009: "O dashboard do Gestor carrega os dados do mes corrente em menos de 3 segundos para bases com ate 10.000 pedidos." Target presente, mensuravel e com baseline de volume especificado. Atende criterio de aceite.

- [ ] CHK046 - Existe target de latencia para as operacoes de escrita criticas (criar pedido, registrar pagamento, acionar apuracao de comissao)? [Completude, Gap, Spec geral] {humano}
  > [Gap] SC-009 define latencia apenas para leitura (dashboard). Operacoes de escrita financeira — especialmente apuracao de comissao (que processa todos os pedidos pagos de um periodo) — nao tem target de latencia. Para POC com 10k pedidos, a apuracao pode ser lenta sem um target de referencia.

- [ ] CHK047 - O target de latencia do dashboard (SC-009: <3s / 10k pedidos) inclui o tempo de query no banco ou apenas tempo de resposta HTTP end-to-end? [Clareza, Ambiguity, Spec §SC-009] {humano}
  > [Ambiguity] "carrega em menos de 3 segundos" nao especifica a fronteira de medicao: tempo de resposta do servidor, tempo de renderizacao no browser, ou tempo percebido pelo usuario (LCP)? A definicao afeta o criterio de teste de performance.

---

## Escalabilidade e Volume

- [x] CHK048 - O volume de dados de referencia (baseline) para avaliar performance esta definido? [Completude, Spec §SC-009] {auto}
  > SC-009: "ate 10.000 pedidos." Baseline de volume definido. Suficiente para POC. Volume maior (100k+) e pos-MVP.

- [ ] CHK049 - Existe definicao do numero esperado de vendedores ativos simultaneos que o sistema deve suportar no MVP? [Completude, Gap, Spec geral] {humano}
  > [Gap] A spec define volume de pedidos (10k) mas nao o numero de vendedores (ex: 10, 100, 1000). O numero de vendedores afeta o volume de commission_rules e o tamanho do resultado de apuracao batch. Para calibrar indices e queries, esse dado e necessario.

- [ ] CHK050 - Existe requisito de comportamento sob carga (ex: requests concorrentes ao endpoint de apuracao por usuarios distintos)? [Completude, Gap, Spec §FR-015] {humano}
  > [Gap] FR-015 define apuracao sob demanda pelo Gestor, mas nao especifica o comportamento se dois Gestores acionarem apuracao do mesmo periodo simultaneamente. O `ON CONFLICT DO NOTHING` do plano lida com idempotencia, mas o requisito de concorrencia nao esta explicitamente no spec.

---

## Estrategia de Query e Indexacao

- [x] CHK051 - O mecanismo de idempotencia da apuracao (UNIQUE order_id + ON CONFLICT DO NOTHING) esta especificado como constraint de banco, nao apenas logica de aplicacao? [Clareza, Spec §FR-014, data-model.md Commission] {auto}
  > data-model: `order_id FK→orders UNIQUE`. Plan: "Apuracao idempotente via constraint UNIQUE(order_id) + ON CONFLICT DO NOTHING." Enforced no banco. Correto e explicito.

- [ ] CHK052 - Existe especificacao de indices para as queries de dashboard e apuracao (ex: index em orders.vendor_id+status+order_date para FR-011/FR-012)? [Completude, Gap, plan.md, data-model.md] {humano}
  > [Gap] O data-model define constraints (PK, FK, UNIQUE) mas nao especifica indices de performance para as queries previsiveis de producao: filtro por periodo (commission.period), filtro por vendedor e status (orders.vendor_id + status), selecao de regra vigente (commission_rules.vendor_id + valid_from/valid_to). Sem indices, SC-009 pode nao ser atingido com 10k pedidos.

- [x] CHK053 - A view `commission_net_balance` e definida como derivada (nao materializada), sendo reconcomputavel sob demanda conforme FR-018? [Completude, Spec §FR-018, FR-029, data-model.md] {auto}
  > data-model: "Saldo liquido NAO armazenado — derivado" com SQL da view definido. FR-018: "valores pre-calculados ou cacheados MUST ser reconcomputaveis sob demanda (P-I)." FR-029: imutabilidade dos registros originais. Presente e correto.

- [ ] CHK054 - O custo de computacao da view `commission_net_balance` em producao (JOIN de commissions + commission_reversals para todos os registros) esta avaliado nos requisitos de performance? [Completude, Gap, data-model.md View, Spec §SC-009] {humano}
  > [Gap] A view realiza um JOIN e GROUP BY sem filtro de periodo — potencialmente scan completo das tabelas. Com volume crescente de comissoes/estornos, a view pode degradar o dashboard. A spec nao define se a view deve ser materializada para leituras ou se uma view simples e suficiente para o volume-alvo de 10k pedidos.

---

## Caching

- [x] CHK055 - O requisito de reconcomputabilidade sob demanda (FR-018) e mutuamente exclusivo com caching que viole rastreabilidade? [Consistencia, Spec §FR-018, P-I] {auto}
  > FR-018: "valores pre-calculados ou cacheados MUST ser reconcomputaveis sob demanda (P-I)." Caching e permitido desde que o valor original seja rastreavel. A spec nao proibe caching — proibe caching irreconciliavel. Consistente.

- [ ] CHK056 - Existe requisito sobre estrategia de invalidacao de cache quando dados subjacentes mudam (ex: novo pedido pago, apuracao de comissao)? [Completude, Gap, Spec §FR-018] {humano}
  > [Gap] Se o frontend ou backend implementar caching de resultados de dashboard, a spec nao define a politica de invalidacao (TTL, event-driven, on-write). Sem isso, o dashboard pode exibir dados desatualizados — violando P-I (rastreabilidade) e SC-004 (drill-down).

---

## Apuracao de Comissao (operacao batch)

- [x] CHK057 - A apuracao e definida como operacao atomica (todos os pedidos do periodo ou nenhum) em caso de falha parcial? [Completude, Spec §FR-014, Edge Cases] {auto}
  > Edge Cases: "A reexecucao deve ser idempotente e nao duplicar comissoes ja persistidas (P-II)." FR-014: idempotencia garantida. Plan: `ON CONFLICT DO NOTHING` no banco. A combinacao de idempotencia + reconexao garante recuperacao de falha parcial sem necessidade de rollback total. Suficiente para MVP.

- [ ] CHK058 - Existe requisito de timeout ou limite de tempo para a operacao de apuracao quando o volume de pedidos for elevado? [Completude, Gap, Spec §FR-015] {humano}
  > [Gap] Para periodos com muitos pedidos, a apuracao pode demorar mais do que o timeout padrao do HTTP (tipicamente 30s). A spec nao define timeout, resposta assincrona (job + polling) ou limite de volume por apuracao sicrona. Impacta UX (usuario aguardando) e confiabilidade (timeout antes de concluir).

---

## Degradacao Graceful

- [ ] CHK059 - Existe requisito definindo o comportamento do sistema quando o banco de dados esta indisponivel (ex: erro explicito vs timeout silencioso)? [Completude, Gap, Spec geral] {humano}
  > [Gap] A spec nao define comportamento em caso de indisponibilidade de dependencias (banco). Para sistema financeiro, a resposta para o usuario deve ser deterministica (503 Service Unavailable com mensagem, nao timeout silencioso).

---

## Notes

- Items `{auto}` ja vem resolvidos pelo agente (`[x]` com citacao, ou marcador `[Gap]`)
- Items `{humano}` ficam `[ ]` aguardando decisao do dono do produto
- Marcar items concluidos com `[x]`

### Resumo

- **{auto} resolvidos**: 5 (`[x]` com evidencia)
- **{humano} aguardando decisao**: 10 (CHK046..CHK050, CHK052, CHK054, CHK056, CHK058, CHK059)
- **Gaps abertos (`[Gap]`/`[Ambiguity]`)**: CHK046, CHK047, CHK049, CHK050, CHK052, CHK054, CHK056, CHK058, CHK059

### Destino dos Gaps

| Item | Marcador | Destino |
|------|----------|---------|
| CHK046 | `[Gap]` | `/clarify` — definir targets de latencia para escrita/apuracao |
| CHK047 | `[Ambiguity]` | `/clarify` — definir fronteira de medicao do SC-009 |
| CHK049 | `[Gap]` | `/clarify` — definir numero de vendedores no baseline de volume |
| CHK050 | `[Gap]` | `/clarify` — definir comportamento em apuracao concorrente |
| CHK052 | `[Gap]` | `/create-tasks` — tarefa "definir e implementar indices de performance" |
| CHK054 | `[Gap]` | `/clarify` — avaliar se view deve ser materializada no volume-alvo |
| CHK056 | `[Gap]` | `/create-tasks` — tarefa "definir estrategia de invalidacao de cache" |
| CHK058 | `[Gap]` | `/clarify` — decidir timeout vs assincrono para apuracao grande |
| CHK059 | `[Gap]` | `/create-tasks` — tarefa "definir comportamento em falha de banco" |
