# Quickstart & Cenarios de Teste — Financial Dashboard

**Feature**: `financial-dashboard`
**Gerado por**: agente-00c, onda-005 (etapa plan)

> Cenarios de teste por fluxo critico (happy path + error case). Cada cenario
> mapeia a User Stories, FRs e Success Criteria. Formato: passos numerados →
> **Expected**.

---

## Cenario 1 — Cadastro e versionamento de vendedor (US1, FR-001..003, P-I)

1. Login como gestor → recebe JWT (role=gestor).
2. POST /vendors `{name, email, commissionPercentage:"5.0000"}`.
3. **Expected**: 201; vendedor ativo; commission_rule v1 com 5.0000,
   `validTo=null`.
4. PUT /vendors/{id}/commission-rule `{percentage:"7.0000"}`.
5. **Expected**: 200; v1 fechada (`validTo=now`), v2 criada (7.0000,
   `validFrom=now`). v1 permanece consultavel (auditoria P-I).
6. PATCH /vendors/{id} `{active:false}` → GET /vendors?active=true.
7. **Expected**: vendedor nao aparece na lista ativa; historico preservado
   (US1 cenario 3).

### Error case 1a — percentual invalido (FR-002)
- POST /vendors com `commissionPercentage:"150"` → **Expected**: 400
  VALIDATION_ERROR (fora de [0,100]). Idem percentual negativo.

### Error case 1b — RBAC (US1 cenario 4, SC-005)
- Login como vendedor → GET /vendors/{outro_id} → **Expected**: 403 FORBIDDEN.

---

## Cenario 2 — Pedido e maquina de estado (US2, FR-006..009, dec-019)

1. Gestor: POST /orders `{vendorId, orderDate, totalCents:150000}`.
2. **Expected**: 201; status=`rascunho`; total armazenado como BIGINT
   centavos (P-III, sem float).
3. POST /orders/{id}/transition `{to:"confirmado"}` → **Expected**: 200,
   status=confirmado, linha em audit_trail (ator, ts UTC, rascunho→confirmado).
4. POST /orders/{id}/transition `{to:"pago"}` → **Expected**: 200, `paidAt`
   setado, trilha registrada.

### Error case 2a — transicao invalida (US2 cenario 3, FR-008)
- Pedido `pago` → POST transition `{to:"rascunho"}` → **Expected**: 422
  STATE_TRANSITION_INVALID. Nenhuma mudanca persistida.

### Error case 2b — total inconsistente com itens (FR-010)
- POST /orders com `totalCents:150000` mas itens somando 100000 →
  **Expected**: 400 VALIDATION_ERROR.

---

## Cenario 3 — Apuracao deterministica e idempotente (US3, FR-011..015, SC-002/003)

1. Vendedor com regra vigente 8.0000. Dois pedidos `pago` no periodo
   `2026-06`: 200000 e 300000 centavos.
2. POST /commissions/calculate `{period:"2026-06"}`.
3. **Expected**: `created:1` por pedido (2 comissoes); valores
   `200000*8% = 16000` e `300000*8% = 24000` centavos; cada uma referencia
   orderId, ruleVersion, appliedPercentage (FR-013, P-I).
4. POST /commissions/calculate `{period:"2026-06"}` (de novo).
5. **Expected**: `created:0, skippedExisting:2` — ZERO duplicatas (SC-003,
   FR-014). Numero de registros identico.

### Error case 3a — pedido nao-pago ignorado (US3 cenario 3, FR-011)
- Pedido `confirmado` no periodo → apurar → **Expected**: nao gera comissao.

### Error case 3b — regra pela data do pedido (US3 cenario 4, FR-012/dec-020)
- Vendedor: regra 5% ate 2026-06-10, 10% a partir de 2026-06-11. Pedido pago
  com `orderDate=2026-06-05`. Apurar em julho → **Expected**: comissao usa 5%
  (vigente na DATA DO PEDIDO, nao da apuracao).

### Error case 3c — arredondamento (P-III, dec-025, SC-007)
- Pedido 33333 centavos, percentual 3.3333% → `33333 * 0.033333 = 1111.07...`
  → **Expected**: arredondado half-up para `1111` centavos, deterministico.

### Error case 3d — periodo vazio (Edge Case)
- Periodo sem pedidos pagos → **Expected**: `created:0` sem erro.

---

## Cenario 4 — Pagamento de comissao com trilha (US4, FR-020..023, SC-006)

1. Financeiro: GET /commissions?status=pendente.
2. POST /commissions/{id}/approve → **Expected**: 200, status=aprovado,
   audit_trail (ator=financeiro, ts, pendente→aprovado).
3. POST /commissions/{id}/pay `{amountCents:16000}` → **Expected**: pago +
   trilha.
4. POST /commissions/{id}/revert (sobre aprovado) `{reason:"ajuste"}` →
   **Expected**: aprovado→pendente, trilha com `reason` (FR-021).

### Error case 4a — reversao sem motivo (FR-021)
- POST revert sem `reason` → **Expected**: 400.

### Error case 4b — vendedor tenta aprovar (US4 cenario 3, FR-023, SC-005)
- Login vendedor → POST /commissions/{id}/approve → **Expected**: 403.

### Error case 4c — escopo de listagem (US4 cenario 4)
- Gestor GET /commissions → ve todas; Vendedor GET /commissions → ve SO as
  proprias.

---

## Cenario 5 — Cancelamento pos-comissao gera estorno (FR-027..029, dec-021)

1. Pedido `pago` com comissao apurada `pendente` (valor 16000).
2. POST /orders/{id}/transition `{to:"cancelado"}`.
3. **Expected**: pedido cancelado; estorno criado `value_cents=-16000`,
   status=`aplicado`; comissao ORIGINAL inalterada (FR-029, P-I);
   `netCents` da comissao = 0 (derivado, view).
4. Repetir com comissao `aprovado`/`pago`: estorno nasce `pendente_aprovacao`
   (FR-028) → Financeiro confirma → `lancado`.
5. **Expected**: comissao original nunca alterada/removida em nenhum caso
   (FR-029, SC verificavel).

---

## Cenario 6 — Dashboards rastreaveis (US5, FR-016..018, SC-004/009)

1. Gestor: GET /dashboard/manager?period=2026-06.
2. **Expected**: totalSalesCents = soma dos pedidos pagos; totalCommissions =
   soma das comissoes; ranking por volume; todos derivados das tabelas (P-I,
   FR-018), reconcomputaveis. Carrega < 3s para 10.000 pedidos (SC-009).
3. Drill-down: do total → GET /commissions → GET /orders/{id} (<= 3 cliques,
   SC-004).
4. Vendedor: GET /dashboard/vendor → **Expected**: SO metricas proprias
   (US5 cenario 3, P-IV). Tentativa de ver outro vendedor = 403.

---

## Cenario 7 — Roundtrip End-to-End (OBRIGATORIO — borda backend↔frontend)

> Chamada REAL ao backend (Playwright dirigindo a UI React), captura do
> payload de resposta e comparacao do shape contra o contrato (api.md).
> Razao: expor drift snake_case/camelCase ou centavos/decimal antes de
> acumular.

1. Subir backend Go + Postgres (migrations aplicadas) + frontend Vite.
2. Playwright: login como gestor na UI → cadastra vendedor → cria pedido →
   transiciona ate `pago` → aciona apuracao → abre dashboard.
3. Interceptar a resposta de `GET /dashboard/manager` (network real).
4. **Expected** — verificar em CADA camada:
   - **UI**: total de vendas exibido = valor esperado formatado (R$).
   - **Network/API**: payload tem chaves **camelCase** (`totalSalesCents`,
     `vendorRanking`) e valores monetarios como **inteiro de centavos**
     (number), percentuais como string decimal — conforme api.md.
   - **DB**: `SELECT` em `orders`/`commissions` confirma BIGINT centavos,
     sem float (P-III, SC-007).
5. **Expected (negativo)**: se o payload viesse em snake_case ou com float, o
   teste FALHA — Zod parse na borda do frontend rejeita o shape divergente.

---

## Mapa de cobertura

| Cenario | User Story | FRs | Success Criteria |
|---------|-----------|-----|------------------|
| 1 | US1 | FR-001..005 | SC-005, SC-008 |
| 2 | US2 | FR-006..010 | SC-007 |
| 3 | US3 | FR-011..015 | SC-002, SC-003, SC-007 |
| 4 | US4 | FR-020..023 | SC-005, SC-006 |
| 5 | — | FR-027..029 | (imutabilidade P-I) |
| 6 | US5 | FR-016..019 | SC-001, SC-004, SC-009 |
| 7 | US5 + borda | FR-016..018, FR-007 | SC-007, SC-009 |
