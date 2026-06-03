# Requirements Quality Checklist: Financial Dashboard

**Purpose**: Validar qualidade geral dos requisitos — clareza, testabilidade, completude, consistencia entre FRs, criterios de aceite mensuravelmente definidos, e ausencia de conflitos internos.
**Created**: 2026-06-03
**Feature**: [spec.md](../spec.md) | [plan.md](../plan.md)

---

## Completude de Criterios de Aceite

- [x] CHK082 - Cada User Story possui criterios de aceite no formato Given/When/Then com valores concretos (nao vagos)? [Completude, Spec §US1..US5] {auto}
  > US1: 4 cenarios GWT com valores concretos (ex: "percentual de comissao, ex: 5,5%"). US2: 4 cenarios. US3: 4 cenarios com calculo concreto (8% de R$5.000 = R$400). US4: 4 cenarios. US5: 4 cenarios. Todos com valores concretos. Presente.

- [x] CHK083 - O Independent Test de cada User Story e de fato independente (nao requer outras stories implementadas)? [Clareza, Spec §US1..US5] {auto}
  > US1 Independent Test: cria/atualiza/desativa vendedor sem pedidos. US2: cria pedido e transiciona sem depender de comissao. US3: calcula comissao com dados de vendedor existente (depende de US1, US2 — mas de forma explicita e aceitavel). US4: aprova/paga comissao pendente sem depender de dashboard. US5: requer dados existentes de US1-US4 — dependencia documentada ("derivado dos dados ja gerados pelas stories anteriores"). Dependencias explicitadas.

- [x] CHK084 - Os Success Criteria (SC-001..SC-009) sao mensuravelmente verificaveis por teste automatizado ou auditoria de schema? [Completude, Spec §Success Criteria] {auto}
  > SC-001: verificavel por cronometro. SC-002/SC-003: verificavel por teste automatizado com dados fixos. SC-004: verificavel por contagem de cliques. SC-005: verificavel por teste de RBAC em todos os endpoints. SC-006: verificavel por query na audit_trail. SC-007: verificavel por inspecao de schema. SC-008: verificavel por inventario. SC-009: verificavel por teste de performance. Todos mensuravelmente definidos.

- [ ] CHK085 - Existe criterio de aceite para a visibilidade do status de Estorno de Comissao no dashboard do Gestor e do Financeiro? [Completude, Gap, Spec §FR-019, FR-027] {humano}
  > [Gap] FR-019 exige que o sistema exiba indicadores de comissoes pendentes de aprovacao e pagamento. FR-027/028 definem estornos com status `pendente_aprovacao`. Mas nao ha criterio de aceite especifico para: o Gestor ve os estornos pendentes no dashboard? O Financeiro recebe indicacao de estornos pendentes para aprovacao? A integracao entre FR-019 e o ciclo de estorno nao esta coberta por um SC especifico.

---

## Clareza e Testabilidade dos FRs

- [x] CHK086 - Todos os FRs que contem MUST sao verificaveis por teste ou inspecao (nao sao puramente aspiracionais)? [Clareza, Spec §Requirements] {auto}
  > Amostra: FR-002 (intervalo [0,100] com mensagem de erro — testavel). FR-008 (4 transicoes validas — testavel por state machine). FR-014 (idempotencia — testavel por SC-003). FR-018 (derivado — testavel por SC-004). FR-029 (imutavel — testavel por trigger). Todos os FRs amostrados sao verificaveis.

- [x] CHK087 - Os Edge Cases listados na spec correspondem a cenarios cobertor por FRs ou criterios de aceite existentes? [Consistencia, Spec §Edge Cases] {auto}
  > Edge Case "percentual zero" → coberto por FR-002 (comissao zero e valida). "dois pedidos identicos" → coberto por FR-006 (IDs unicos). "crash durante apuracao" → coberto por FR-014 (idempotencia). "cancelamento pos-apuracao" → coberto por FR-027/028. "nenhum pedido no periodo" → coberto pelo Independent Test de US3. "fuso horario" → coberto por Edge Case com resolucao UTC. Todos mapeados.

- [ ] CHK088 - Os FRs distinguem claramente entre o que o Gestor PODE fazer e o que o SISTEMA faz automaticamente (ex: criacao de estorno)? [Clareza, Ambiguity, Spec §FR-027] {humano}
  > [Ambiguity] FR-027: "o sistema MUST criar automaticamente um registro de Estorno de Comissao." Claro que e automatico. Mas FR-015 diz: "apuracao de comissoes MUST ser executavel por periodo mensal, sob demanda pelo Gestor/Admin." Existe ambiguidade sobre o fluxo: o Gestor cancela um pedido pago → sistema cria estorno automaticamente? Ou ha alguma confirmacao intermediaria? O fluxo do cancelamento ate o estorno deveria ser mais explicitamente especificado como sequencia de acoes.

- [ ] CHK089 - Os FRs numerados sequencialmente (FR-001 a FR-029) estao ordenados de forma que dependencias entre FRs sao rastreavels? [Consistencia, Spec §Requirements] {humano}
  > [Ambiguity] Os FRs estao agrupados por dominio (bom), mas a numeracao tem saltos notaveis: FR-015 seguido diretamente por FR-016 (sem FR-015.1 etc.) mas FR-027/028/029 aparecem em Dominio 3.bis (depois de Dominio 5 na numeracao). Um leitor pode nao perceber que FR-027-029 existem ao analisar o Dominio 3. Reorganizacao ou indice de FRs por dominio melhoraria rastreabilidade.

---

## Consistencia Interna

- [x] CHK090 - A maquina de estado de comissao (payment_status: pendente/aprovado/pago) e consistente entre spec, data-model e criterios de aceite? [Consistencia, Spec §FR-020, data-model.md Commission] {auto}
  > Spec FR-020: 3 estados + 3 transicoes validas. data-model Commission: `payment_status CHECK in (pendente,aprovado,pago)`. US4 Acceptance Scenarios cobrem pendente→aprovado, aprovado→pago, e RBAC. Consistente.

- [x] CHK091 - O status do Estorno de Comissao (aplicado | pendente_aprovacao | aprovado | lancado) e consistente entre spec FR-028 e data-model? [Consistencia, Spec §FR-028, data-model.md CommissionReversal] {auto}
  > FR-028 menciona "pendente_aprovacao" e o fluxo de aprovacao do Financeiro. data-model: `status CHECK in (aplicado, pendente_aprovacao, aprovado, lancado)` com 4 estados. Consistente. (Nota: spec menciona `pendente_aprovacao` mas nao define `lancado` explicitamente como estado — data-model e mais completo que a spec aqui, o que e aceitavel.)

- [x] CHK092 - A restricao de que Comissao so e calculada sobre pedidos com status `pago` (FR-011) e consistente com a maquina de estado de pedido (FR-008)? [Consistencia, Spec §FR-011, FR-008] {auto}
  > FR-011: "SOMENTE sobre pedidos com status pago." FR-008: `pago` e um dos 4 estados validos. US3 Acceptance Scenario 3: "pedido com status confirmado NAO entra no calculo." Consistente.

- [ ] CHK093 - Existe consistencia entre FR-019 (exibir indicadores de comissoes pendentes) e FR-016 (dashboard do Gestor) quanto ao que exatamente o Gestor ve sobre pagamentos? [Consistencia, Ambiguity, Spec §FR-016, FR-019] {humano}
  > [Ambiguity] FR-016 define o dashboard do Gestor como "total de vendas, total de comissoes apuradas e lista de vendedores com volumes." FR-019 adiciona "indicadores de status de comissoes pendentes de aprovacao e pagamento." Nao e claro se FR-019 e uma secao DO dashboard de FR-016 ou uma tela separada. A ausencia de mockup ou wireframe de referencia deixa margem para implementacoes divergentes.

- [x] CHK094 - A relacao entre `commission_payments` (Pagamento de Comissao como agregado) e `commissions.payment_status` esta especificada sem conflito? [Consistencia, Spec §Key Entities, data-model.md CommissionPayment] {auto}
  > data-model: `commission_payments` e "o agregado opcional de lote. No MVP, o ciclo de aprovacao/pagamento e modelado primariamente em commissions.payment_status + audit_trail." A relacao esta documentada como: payment_status em commissions e a fonte de verdade individual; commission_payments e o agregador de lote opcional. Sem conflito — hierarquia clara.

---

## Escopo e Limites

- [x] CHK095 - A secao "Fora de Escopo" esta suficientemente clara para prevenir scope creep durante implementacao? [Completude, Spec §Fora de Escopo] {auto}
  > 10 itens explicitamente fora de escopo com linguagem clara: ERP, NF-e, multi-moeda, multi-tenant, metas, comissao por faixa, exportacao, notificacoes, agendamento automatico, gateway de pagamento. Suficientemente especifica para prevenir scope creep.

- [x] CHK096 - Os requisitos marcados como "pos-MVP" (ex: apuracao automatica periodica) estao claramente distinguidos dos requisitos do MVP? [Clareza, Spec §FR-015] {auto}
  > FR-015: "apuracao automatica periodica e desejavel pos-MVP." Notas apos Dominio 3 explicitam decisoes de infraestrutura como pos-MVP. Claro.

- [x] CHK097 - A lista de ambiguidades resolvidas cobre todos os pontos que eram `[A VALIDAR]` na spec original? [Completude, Spec §Resolved Ambiguities] {auto}
  > Resolved Ambiguities lista A-001 a A-006, todos com status "Resolvido" e decisao referenciada (dec-018..dec-021). Nota no cabecalho da spec: "Todos os itens [A VALIDAR] foram resolvidos em clarify (onda-004)." Consistente.

---

## Dependencias e Premissas

- [x] CHK098 - As dependencias entre User Stories (US1 → US2 → US3 → US4) estao explicitadas na justificativa de prioridade? [Completude, Spec §US1..US5] {auto}
  > Cada US tem "Why this priority" documentando dependencias: US2 depende de US1 (vendedores), US3 depende de US1+US2, US4 depende de US3, US5 depende de US1-US4. Encadeamento explicitado.

- [ ] CHK099 - As premissas tecnicas (Go 1.22+, PostgreSQL 16, React 18) sao explicitas e validadas como premissas do projeto, nao assumidas silenciosamente? [Completude, Assumption, plan.md Technical Context] {humano}
  > [Assumption] Plan Technical Context lista as versoes especificas como stack fixada pela Constitution. Mas nao ha verificacao de que o ambiente de desenvolvimento/producao suporta essas versoes. Para POC/greenfield, pode ser premissa valida — mas deveria ser documentada como premissa explicitamente validada (ex: "disponivel no ambiente alvo").

- [x] CHK100 - A ausencia de integracoes externas (ERP, gateway de pagamento, scheduler) esta documentada como premissa do MVP? [Completude, Spec §Fora de Escopo] {auto}
  > "Integracao com ERP / sistemas de contabilidade externa", "gateway de pagamento (nao ha PCI-DSS no MVP)", "Agendamento automatico de apuracao mensal" — todos explicitamente fora de escopo. Premissa documentada.

---

## Notes

- Items `{auto}` ja vem resolvidos pelo agente (`[x]` com citacao, ou marcador `[Gap]`)
- Items `{humano}` ficam `[ ]` aguardando decisao do dono do produto
- Marcar items concluidos com `[x]`

### Resumo

- **{auto} resolvidos**: 14 (`[x]` com evidencia)
- **{humano} aguardando decisao**: 5 (CHK085, CHK088, CHK089, CHK093, CHK099)
- **Gaps/Ambiguities abertos**: CHK085(Gap), CHK088(Ambiguity), CHK089(Ambiguity), CHK093(Ambiguity), CHK099(Assumption)

### Destino dos Gaps

| Item | Marcador | Destino |
|------|----------|---------|
| CHK085 | `[Gap]` | `/create-tasks` — tarefa "definir visibilidade de estornos pendentes no dashboard Gestor/Financeiro" |
| CHK088 | `[Ambiguity]` | `/clarify` — detalhar sequencia de acoes no fluxo cancelamento→estorno |
| CHK089 | `[Ambiguity]` | `/create-tasks` — tarefa "reorganizar numeracao FRs ou adicionar indice por dominio" |
| CHK093 | `[Ambiguity]` | `/clarify` — definir se indicadores de FR-019 sao parte do dashboard FR-016 ou tela separada |
| CHK099 | `[Assumption]` | `/create-tasks` — tarefa "documentar premissas de ambiente como pre-requisitos de setup" |
