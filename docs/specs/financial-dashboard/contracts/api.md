# API Contracts — Financial Dashboard

**Feature**: `financial-dashboard`
**Gerado por**: agente-00c, onda-005 (etapa plan)
**Base URL**: `/api/v1`
**Auth**: `Authorization: Bearer <JWT>` em TODAS as rotas exceto `POST /auth/login`.
**Convencao de payload**: camelCase (request e response). Valores monetarios
trafegam como **inteiro de centavos** (`number`, ex: `150000` = R$ 1.500,00).
Percentuais como `string` decimal (`"5.5000"`) para preservar precisao na
borda (P-III). Datas em ISO-8601 UTC.

> RBAC (P-IV, deny-by-default): cada endpoint declara papeis autorizados.
> Papel ausente da lista → `403 Forbidden`. Token ausente/invalido → `401`.
> Vendedor so acessa recursos do proprio `vendor_id` (escopo server-side,
> SC-005) — re-validado contra o recurso, nao confiando so no token.

---

## Erros (padrao)

```json
{ "error": { "code": "VALIDATION_ERROR", "message": "percentage must be in [0,100]", "details": [] } }
```

| HTTP | code | Quando |
|------|------|--------|
| 400 | VALIDATION_ERROR | payload invalido / transicao de estado invalida |
| 401 | UNAUTHENTICATED | token ausente/expirado |
| 403 | FORBIDDEN | papel sem permissao / escopo de vendedor violado (P-IV) |
| 404 | NOT_FOUND | recurso inexistente |
| 409 | CONFLICT | estado conflitante (ex: email duplicado) |
| 422 | STATE_TRANSITION_INVALID | transicao rejeitada pela maquina de estado |

---

## 1. Auth

### POST /auth/login  — publico
Req: `{ "email": "...", "password": "..." }`
Resp 200: `{ "accessToken": "<jwt>", "refreshToken": "<jwt>", "role": "gestor", "vendorId": null }`
Resp 401: credenciais invalidas.

### POST /auth/refresh — autenticado (refresh token)
Resp 200: `{ "accessToken": "<jwt>" }`

---

## 2. Vendedores  (Dominio 1 — FR-001..005)

### POST /vendors — **gestor**
Req: `{ "name": "...", "email": "...", "commissionPercentage": "5.5000" }`
Cria vendedor + commission_rule v1 (`validFrom = now`). Resp 201:
`{ "id", "name", "email", "active": true, "currentPercentage": "5.5000" }`
Erros: 400 (percentage fora de [0,100], FR-002), 409 (email duplicado).

### GET /vendors — **gestor**  (Vendedor NAO lista outros — P-IV)
Query: `?active=true`. Resp 200: `[ { "id","name","email","active","currentPercentage" } ]`

### GET /vendors/{id} — **gestor**; **vendedor** so o proprio (SC-005)
Resp 200 vendedor; 403 se vendedor pede outro id.

### PATCH /vendors/{id} — **gestor**
Req (parcial): `{ "name?", "email?", "active?" }`. Nao altera percentual
aqui (ver rota dedicada). Resp 200.

### PUT /vendors/{id}/commission-rule — **gestor**  (FR-003)
Req: `{ "percentage": "7.0000" }`
Fecha versao vigente (`validTo = now`) + cria nova versao. Resp 200:
`{ "vendorId", "newVersion": 2, "percentage": "7.0000", "validFrom": "..." }`
Erros: 400 (fora de [0,100]).

### DELETE /vendors/{id} — **gestor**  (FR-005, LGPD)
Anonimiza PII (name/email→token, `anonymizedAt` setado), preserva registros
financeiros. Resp 200: `{ "id", "anonymizedAt": "..." }`.
NAO faz hard-delete (P-I prevalece).

---

## 3. Pedidos  (Dominio 2 — FR-006..010)

### POST /orders — **gestor**
Req:
```json
{ "vendorId": "...", "orderDate": "2026-06-01T00:00:00Z", "totalCents": 150000,
  "items": [ { "description": "Plano Pro", "quantity": 1, "unitPriceCents": 150000 } ] }
```
`status` inicial = `rascunho`. Se `items` informado, `totalCents` MUST =
SUM(line totals) (FR-010) senao 400. Resp 201 com pedido + status.

### GET /orders — **gestor** (todos); **vendedor** (so os proprios — SC-005)
Query: `?status=pago&vendorId=&from=&to=`. Resp 200: lista.

### GET /orders/{id} — **gestor**; **vendedor** so proprio.
Resp 200 com itens + `transitions` (trilha — P-I).

### POST /orders/{id}/transition — **gestor**  (FR-008, maquina de estado)
Req: `{ "to": "confirmado" }`  (valores: confirmado|pago|cancelado)
Valida transicao (rascunho→confirmado, confirmado→pago, confirmado→cancelado,
pago→cancelado). Grava audit_trail (FR-009). Transicao `pago→cancelado` com
comissao existente DISPARA estorno (FR-027) na mesma transacao.
Resp 200: `{ "id", "status", "transitionedAt" }`.
Resp 422 STATE_TRANSITION_INVALID: ex. `pago→rascunho`.

---

## 4. Apuracao de Comissao  (Dominio 3 — FR-011..015)

### POST /commissions/calculate — **gestor**  (FR-015, sob demanda)
Req: `{ "period": "2026-06" }`
Apura comissoes de TODOS os pedidos `pago` com `orderDate` no periodo,
selecionando a regra vigente na DATA DO PEDIDO (FR-012, dec-020).
**Idempotente** (`ON CONFLICT (orderId) DO NOTHING`, FR-014/SC-003):
reexecutar nao duplica. Resp 200:
```json
{ "period": "2026-06", "created": 12, "skippedExisting": 3, "totalValueCents": 480000 }
```
Pedidos nao-`pago` ignorados (FR-011). Periodo sem pedidos pagos → `created: 0`
sem erro (Edge Case).

### GET /commissions — **gestor** (todas); **vendedor** (so as proprias, SC-005); **financeiro** (todas, para pagamento)
Query: `?period=2026-06&vendorId=&status=pendente`. Resp 200:
```json
[ { "id","orderId","vendorId","valueCents","netCents","appliedPercentage",
    "ruleVersion","period","paymentStatus","calculatedAt" } ]
```
`netCents` = valor liquido derivado (comissao + estornos, FR-029/view).

### GET /commissions/{id} — escopo por papel.
Resp 200 com cadeia de auditoria: pedido origem, percentual, versao de
regra, transicoes de pagamento, estornos (SC-004, P-I).

---

## 5. Pagamentos de Comissao  (Dominio 5 — FR-020..023)

### POST /commissions/{id}/approve — **financeiro**  (FR-020)
`pendente → aprovado`. Grava audit_trail (ator, ts, from/to — FR-021).
Resp 200. 403 se papel != financeiro (SC-005; vendedor/gestor negados na
operacao de pagamento, FR-023).

### POST /commissions/{id}/pay — **financeiro**  (FR-020)
`aprovado → pago`. Req: `{ "amountCents": 40000 }`. Grava audit. Resp 200.

### POST /commissions/{id}/revert — **financeiro**  (FR-020)
`aprovado → pendente`. Req: `{ "reason": "ajuste de valor" }` (motivo
OBRIGATORIO — FR-021). Resp 200; 400 se `reason` ausente.

### GET /commission-reversals — **financeiro**, **gestor**  (FR-028)
Query: `?status=pendente_aprovacao`. Lista estornos para conferencia.

### POST /commission-reversals/{id}/confirm — **financeiro**  (FR-028)
Confirma lancamento de estorno `pendente_aprovacao` → `aprovado` → `lancado`.
Resp 200. (Estornos de comissao `pendente` ja nascem `aplicado`, sem este
passo.)

---

## 6. Dashboards  (Dominio 4 — FR-016..019)

### GET /dashboard/manager — **gestor**  (FR-016)
Query: `?period=2026-06` ou `?from=&to=`. Resp 200:
```json
{ "period": "2026-06",
  "totalSalesCents": 5000000,
  "totalCommissionsCents": 400000,
  "pendingApprovalCount": 4,
  "vendorRanking": [ { "vendorId","vendorName","salesCents","commissionsCents" } ] }
```
Todos os valores derivados deterministicamente das tabelas transacionais +
view (FR-018, P-I). Drill-down via GET /orders e GET /commissions (SC-004).

### GET /dashboard/vendor — **vendedor** (so o proprio, SC-005)  (FR-017)
Escopo forcado ao `vendorId` do token (re-validado server-side). Resp 200:
```json
{ "period":"2026-06", "mySalesCents": 1500000, "myCommissionsCents": 120000,
  "myPaymentsStatus": { "pendente": 2, "aprovado": 1, "pago": 3 } }
```
NUNCA expoe dados de outro vendedor (P-IV).

---

## Matriz RBAC (P-IV, deny-by-default)

| Endpoint grupo | gestor | vendedor | financeiro |
|----------------|:------:|:--------:|:----------:|
| /vendors (CRUD, rule) | ALLOW | self GET only | DENY |
| /orders (CRUD, transition) | ALLOW | self GET only | DENY |
| /commissions/calculate | ALLOW | DENY | DENY |
| /commissions (GET) | ALL | self only | ALL |
| /commissions/{approve,pay,revert} | DENY | DENY | ALLOW |
| /commission-reversals/* | GET only | DENY | ALLOW |
| /dashboard/manager | ALLOW | DENY | DENY |
| /dashboard/vendor | DENY | self only | DENY |

Qualquer combinacao nao marcada ALLOW = `403` (deny-by-default, FR-026).
Enforcement no backend (FR-025); UI e complementar, nao barreira (P-IV).
