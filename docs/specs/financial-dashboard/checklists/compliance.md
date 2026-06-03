# Compliance Checklist: Financial Dashboard

**Purpose**: Validar qualidade dos requisitos de conformidade — precisao monetaria (P-III), auditabilidade financeira (P-I), idempotencia de calculos (P-II), LGPD/privacidade (P-V) e integridade de dados financeiros.
**Created**: 2026-06-03
**Feature**: [spec.md](../spec.md) | [plan.md](../plan.md) | [data-model.md](../data-model.md)

---

## Precisao Monetaria (P-III — NON-NEGOTIABLE)

- [x] CHK060 - Existe requisito explicitando que NENHUM valor monetario e armazenado como ponto flutuante (float/double/real)? [Completude, Spec §FR-007, SC-007, P-III] {auto}
  > FR-007: "sem ponto flutuante binario." SC-007: "auditoria do schema de banco confirma ausencia de colunas float em campos monetarios." data-model: todas as colunas monetarias sao BIGINT (centavos). P-III NON-NEGOTIABLE. Presente e verificavel.

- [x] CHK061 - A unidade de representacao monetaria (centavos em inteiro) esta especificada em todos os artefatos relevantes (spec, plan, data-model, contratos de API)? [Consistencia, Spec §FR-007, plan.md Convencoes de Borda] {auto}
  > Plan Convencoes de Borda: "Valores monetarios (payload): inteiro de centavos (number)." data-model: BIGINT centavos em todas as colunas. SC-007 como criterio de aceite. Consistente em todos os artefatos.

- [x] CHK062 - A regra de arredondamento (half-up) esta definida, centralizada e documentada com o nome da funcao responsavel? [Clareza, plan.md Technical Context, research dec-025] {auto}
  > Plan: "Arredondamento: half-up, centralizado em money.RoundCommission (research dec-025)." Nome da funcao especificado. Centralizacao documentada. Presente.

- [ ] CHK063 - Existe criterio de aceite mensuravel que valide que zero colunas float existem no schema de banco (SC-007) — ex: teste de schema automatizado? [Clareza, Spec §SC-007, data-model.md] {humano}
  > SC-007 menciona "auditoria do schema de banco confirma ausencia de colunas float" mas nao define o mecanismo de verificacao (teste automatizado em CI, script manual). Para ser realmente mensuravel, o criterio deveria especificar: "teste X em Y arquivos verifica Z." Plan menciona teste de schema (varre information_schema.columns) — mas nao esta como requisito formal na spec.

- [x] CHK064 - A formula de calculo de comissao `comissao = valor_total_pedido_pago * percentual_vigente` esta especificada com o ponto exato de arredondamento (quando e onde half-up e aplicado)? [Clareza, Spec §FR-012, research dec-025] {auto}
  > FR-012: formula explicita. Plan/research dec-025: `RoundCommission(order.total_cents * applied_percentage / 100)` com half-up centralizado. data-model Commission: `value_cents BIGINT CHECK >= 0 — arredondado half-up (dec-025)`. Presente e preciso.

---

## Auditabilidade Financeira (P-I — NON-NEGOTIABLE)

- [x] CHK065 - Todos os registros financeiros criticos sao imutaveis pos-criacao por mecanismo tecnico (nao apenas por convencao de codigo)? [Completude, Spec §P-I, data-model.md] {auto}
  > data-model: "trigger BEFORE UPDATE OR DELETE → RAISE EXCEPTION" em tabelas financeiras. P-I NON-NEGOTIABLE. Enforced no banco, nao apenas no codigo. Presente.

- [x] CHK066 - A trilha de auditoria captura TODOS os 4 dados obrigatorios (ator, timestamp UTC, estado anterior, estado novo) em toda transicao de estado? [Completude, Spec §FR-009, FR-021, SC-006] {auto}
  > FR-009 e FR-021 exigem: ator, timestamp UTC, estado anterior/novo. SC-006: "Toda transicao de estado de pagamento de comissao tem trilha com ator, timestamp e estado anterior/novo — sem excecao." data-model AuditTrail: actor_user_id, occurred_at, from_state, to_state, reason. Presente.

- [x] CHK067 - O requisito de rastreabilidade de valor de dashboard ate pedidos individuais esta especificado com criterio mensuravel (SC-004: <= 3 cliques)? [Clareza, Spec §SC-004, FR-018] {auto}
  > SC-004: "Qualquer valor exibido em dashboard pode ser rastreado ate os pedidos individuais de origem em no maximo 3 cliques (auditabilidade, P-I)." FR-018: "derivado deterministicamente dos registros transacionais." Criterio mensuravel presente.

- [x] CHK068 - O registro de Estorno de Comissao e definido como entidade imutavel separada (nao modificacao do registro original)? [Completude, Spec §FR-029, FR-027, dec-021] {auto}
  > FR-029: "registros originais de Comissao MUST NOT ser alterados ou removidos em nenhum cenario de cancelamento." FR-027: entidade Estorno separada com valor negativo. data-model: `commission_reversals` como tabela distinta com trigger de imutabilidade. Presente e correto.

- [ ] CHK069 - Existe requisito de que a trilha de auditoria registra TAMBEM alteracoes de regra de comissao (criacao de nova versao por alteracao de percentual)? [Completude, Gap, Spec §FR-003, data-model.md AuditTrail] {humano}
  > [Gap] FR-003 exige nova versao da regra a cada alteracao de percentual. data-model: `audit_trail.entity_type` lista 'commission_rule'. Mas FR-003 nao menciona explicitamente a gravacao na audit_trail para alteracoes de regra — apenas versionamento em commission_rules. A consistencia entre FR-003 e o entity_type 'commission_rule' na audit_trail deve ser confirmada como requisito explicito.

---

## Idempotencia e Integridade de Calculo (P-II — NON-NEGOTIABLE)

- [x] CHK070 - O requisito de idempotencia da apuracao e verificavel por criterio de aceite mensuravel (SC-003)? [Completude, Spec §SC-003, FR-014] {auto}
  > SC-003: "Reprocessar a apuracao do mesmo periodo duas vezes produz exatamente o mesmo numero de registros de comissao — sem duplicatas." FR-014: idempotencia MUST. UNIQUE(order_id) + ON CONFLICT DO NOTHING em data-model. Criterio presente e testavel.

- [x] CHK071 - A selecao do percentual vigente pela DATA DO PEDIDO (nao da apuracao) esta especificada sem ambiguidade e com criterio de aceite? [Clareza, Spec §FR-012, US3.4, dec-020] {auto}
  > FR-012: "percentual_vigente e o percentual do vendedor vigente na DATA DO PEDIDO (campo data_do_pedido)." US3.4 Acceptance Scenario: descreve o comportamento quando percentual muda no meio do mes. Clarifications A-005 confirma com dec-020. Sem ambiguidade.

- [x] CHK072 - Existe criterio de aceite para verificar que o determinismo do calculo (SC-002) e testavel com dados fixos? [Completude, Spec §SC-002] {auto}
  > SC-002: "Dado um conjunto de pedidos pagos com valores e percentuais conhecidos, o sistema calcula a comissao com resultado identico em 100% das execucoes." US3 Independent Test especifica valores exatos (10%, 3 pedidos de R$1.000 = R$300 comissao). Testavel.

- [ ] CHK073 - Existe requisito definindo o comportamento quando a apuracao de um periodo falha parcialmente (ex: erro de banco apos 50% dos pedidos processados) — e a retomada e idempotente nesse cenario? [Completude, Gap, Spec §FR-014, Edge Cases] {humano}
  > [Gap] Edge Cases mencionam crash durante processamento e retomada idempotente. FR-014 garante idempotencia via UNIQUE + ON CONFLICT. Mas o requisito nao especifica: (a) se a apuracao usa uma transacao unica (tudo-ou-nada) ou commits incrementais, e (b) se um estado intermediario parcialmente comitado e recuperavel ou requer reprocessamento completo. Isso afeta a implementacao e a confiabilidade.

---

## LGPD e Privacidade (P-V)

- [x] CHK074 - O inventario de dados pessoais (PII) esta completo e mapeado a finalidades especificas do MVP? [Completude, Spec §FR-004, SC-008, data-model.md Vendor] {auto}
  > FR-004: 4 campos pessoais (nome, email, id, percentual via commission_rules) com finalidades mapeadas. SC-008: "inventario de dados pessoais mapeia cada campo a uma finalidade explicita do MVP." Minizimacao P-V documentada. Presente.

- [x] CHK075 - A politica de exclusao de dados pessoais (anonimizacao preservando registros financeiros) reconcilia corretamente FR-005 com P-I? [Consistencia, Spec §FR-005, data-model.md Vendor] {auto}
  > FR-005: "preservando os registros financeiros anonimizados necessarios para auditoria (P-I prevalece)." data-model: `anonymized_at` + name/email → token. Reconciliacao explicita. Consistente.

- [x] CHK076 - A vinculacao obrigatoria entre Usuario com papel Vendedor e entidade Vendedor esta especificada no modelo de dados? [Completude, Spec §Key Entities: Usuario, data-model.md User] {auto}
  > Spec Key Entities: "Um Vendedor-usuario MUST estar vinculado a uma entidade Vendedor para restricoes de escopo de dados." data-model User: CHECK `(role = 'vendedor') = (vendor_id IS NOT NULL)`. Enforced no banco. Presente.

- [ ] CHK077 - Existe definicao do ator autorizado a solicitar exclusao (anonimizacao) de dados pessoais de vendedor — e do fluxo de aprovacao? [Completude, Gap, Spec §FR-005] {humano}
  > [Gap] FR-005 define o RESULTADO da exclusao (anonimizacao) mas nao define: (a) quem pode solicitar (o proprio vendedor via self-service? apenas o Admin?), (b) se ha fluxo de aprovacao (Admin confirma antes de anonimizar?), e (c) como o vendedor comprova a solicitacao. Requisito necessario para conformidade LGPD Arts. 17-20.

- [ ] CHK078 - Existe requisito de log/trilha da solicitacao de exclusao LGPD (quem solicitou, quando, quem executou, quando)? [Completude, Gap, Spec §FR-005, P-I] {humano}
  > [Gap] A spec define anonimizacao mas nao menciona que a operacao em si deve ser auditada na audit_trail (ou em log especifico de LGPD). Para conformidade com LGPD Art. 48 (comunicacao de incidentes), saber QUANDO dados foram anonimizados e por quem pode ser necessario.

---

## Estorno de Comissao

- [x] CHK079 - O fluxo completo de estorno (pedido pago → cancelado → estorno criado → aprovacao do Financeiro se necessario → lancamento) esta especificado sem ambiguidade? [Completude, Spec §FR-027, FR-028, dec-021, data-model.md CommissionReversal] {auto}
  > FR-027: criacao automatica de estorno ao cancelar pedido pago com comissao. FR-028: bifurcacao por status da comissao (pendente → aplicado; aprovado/pago → pendente_aprovacao). data-model: 4 status do estorno (aplicado | pendente_aprovacao | aprovado | lancado). Fluxo completo e sem ambiguidade.

- [x] CHK080 - O calculo do saldo liquido de comissao (comissao + estornos) e derivado computacionalmente, nao armazenado como campo editavel? [Clareza, Spec §FR-029, data-model.md View] {auto}
  > FR-029: "O saldo liquido de uma comissao e derivado computacionalmente (comissao + estornos associados), nao armazenado como campo editavel." data-model: view `commission_net_balance` com SQL explicito. Presente e correto.

- [ ] CHK081 - Existe criterio de aceite para verificar que o estorno e criado ATOMICAMENTE com o cancelamento do pedido (na mesma transacao)? [Completude, Gap, Spec §FR-027] {humano}
  > [Gap] FR-027 define que o estorno MUST ser criado quando o pedido pago e cancelado, mas nao especifica se e na mesma transacao de banco. Se forem transacoes separadas, um crash entre o cancelamento e a criacao do estorno deixaria o sistema em estado inconsistente (pedido cancelado sem estorno). Atomicidade transacional deveria ser requisito explicito.

---

## Notes

- Items `{auto}` ja vem resolvidos pelo agente (`[x]` com citacao, ou marcador `[Gap]`)
- Items `{humano}` ficam `[ ]` aguardando decisao do dono do produto
- Marcar items concluidos com `[x]`

### Resumo

- **{auto} resolvidos**: 14 (`[x]` com evidencia)
- **{humano} aguardando decisao**: 8 (CHK063, CHK069, CHK073, CHK077, CHK078, CHK081 + parciais)
- **Gaps abertos (`[Gap]`)**: CHK063, CHK069, CHK073, CHK077, CHK078, CHK081

### Destino dos Gaps

| Item | Marcador | Destino |
|------|----------|---------|
| CHK063 | `[Gap]` | `/create-tasks` — tarefa "implementar teste de schema SC-007 em CI" |
| CHK069 | `[Gap]` | `/clarify` — confirmar que FR-003 grava em audit_trail |
| CHK073 | `[Gap]` | `/clarify` — definir se apuracao e transacao unica ou commits incrementais |
| CHK077 | `[Gap]` | `/clarify` — definir ator autorizado e fluxo de solicitacao de exclusao LGPD |
| CHK078 | `[Gap]` | `/create-tasks` — tarefa "adicionar trilha de operacoes LGPD" |
| CHK081 | `[Gap]` | `/clarify` — confirmar atomicidade transacional do cancelamento + estorno |
