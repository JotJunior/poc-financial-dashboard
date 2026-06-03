# Implementation Plan — Financial Dashboard

**Feature**: `financial-dashboard`
**Created**: 2026-06-03
**Gerado por**: agente-00c, onda-005 (etapa plan, modo autonomo)
**Predecessor**: spec.md (Clarified, 29 FRs); Constitution v1.0.0 (5 principios
NON-NEGOTIABLE/RBAC/LGPD); Briefing
**Artefatos irmaos**: research.md (Phase 0), data-model.md (Phase 1),
contracts/api.md (Phase 1), quickstart.md (Phase 1)

---

## Summary

Sistema de gestao de vendedores, pedidos, apuracao automatica de comissoes,
controle de pagamentos e dashboards de performance. Requisito primario: motor
financeiro **correto, auditavel, deterministico e idempotente** (prioridade da
constitution: corretude/auditabilidade > UX > velocidade > escopo).

Abordagem tecnica (de research.md): API REST em Go (chi + pgx) sobre
PostgreSQL 16, com **valores monetarios em inteiro de centavos** (`int64` /
`BIGINT`) e **percentuais em decimal** (`NUMERIC(7,4)` / `shopspring/decimal`)
— zero ponto flutuante (P-III). Regras de comissao **versionadas por intervalo
temporal** (selecao pela data do pedido, FR-012). Apuracao **idempotente** via
constraint `UNIQUE(order_id)` + `ON CONFLICT DO NOTHING`. Tudo financeiro e
**append-only** com trigger anti-UPDATE/DELETE e **trilha de auditoria**
transacional. RBAC **deny-by-default** server-side via middleware + escopo de
vendedor. Frontend React+TS+Vite com graficos (recharts) e validacao Zod nas
bordas. Estorno de comissao como **entidade imutavel separada** (saldo liquido
derivado, nunca mutado — FR-029).

---

## Constitution Check

*GATE: passou antes do Phase 0; re-checado apos Phase 1 (secao final).*

| Principio | Status | Notas |
|-----------|--------|-------|
| I. Auditabilidade Financeira Total (NON-NEGOTIABLE) | PASS | Comissao referencia order_id + rule_id + applied_percentage imutaveis (FR-013); audit_trail append-only transacional (FR-009/021); dashboards derivados deterministicamente + drill-down (FR-018, SC-004). Triggers anti-UPDATE/DELETE em tabelas financeiras. |
| II. Integridade do Calculo (NON-NEGOTIABLE) | PASS | Calculo deterministico (decimal, dec-025); idempotencia por `UNIQUE(order_id)` + `ON CONFLICT` (FR-014, SC-003); regra versionada selecionada pela data do pedido (FR-012); comissao so sobre `pago` (FR-011). |
| III. Precisao Monetaria sem Float (NON-NEGOTIABLE) | PASS | Dinheiro = BIGINT centavos; percentual = NUMERIC(7,4)/decimal; zero colunas float (SC-007); arredondamento half-up unico e documentado (dec-025). |
| IV. RBAC Deny-by-Default | PASS | Middleware server-side exige token; matriz RBAC explicita (api.md); escopo de vendedor re-validado contra recurso (SC-005); UI complementar, nao barreira (FR-025). |
| V. LGPD e Minimizacao | PASS | 4 campos pessoais mapeados a finalidade (SC-008, FR-004); exclusao = anonimizacao preservando financeiro (FR-005, dec-031); acesso a PII por papel (P-IV). |

**Resultado**: 5/5 PASS. Nenhuma violacao NON-NEGOTIABLE. Prosseguir.

---

## Technical Context

| Campo | Valor | Fonte |
|-------|-------|-------|
| Linguagem backend | Go 1.22+ | Constitution (stack fixada) |
| Linguagem frontend | TypeScript (React 18 + Vite) | Constitution |
| Banco | PostgreSQL 16 | Constitution |
| Router HTTP | chi v5 | research dec-032 |
| Driver DB | pgx/v5 (SQL explicito, sem ORM) | research dec-032 |
| Migrations | golang-migrate | research dec-032 |
| Decimal | shopspring/decimal (percentual) | research dec-024 |
| Auth | JWT HS256 stateless + refresh | research dec-026 (FR-024) |
| Logs | slog (estruturado) | Padroes Tecnicos (P-I) |
| Frontend data | @tanstack/react-query | research dec-032 |
| Frontend charts | recharts | research dec-032 |
| Validacao borda | zod (frontend), validacao manual+CHECK (backend) | research dec-032 |
| Testes | Go testing+testify; vitest; Playwright (roundtrip) | quickstart §7 |
| Arredondamento | half-up, centralizado em money.RoundCommission | research dec-025 |
| Fuso | UTC interno/persistencia; exibicao local no frontend | spec Edge Cases |

NEEDS CLARIFICATION restantes: **0** (todos resolvidos em research.md).

---

## Project Structure

### Documentacao (feature dir) — EXISTENTE

```
docs/specs/financial-dashboard/
├── spec.md            (existente, Clarified)
├── plan.md            (este arquivo)
├── research.md        (Phase 0)
├── data-model.md      (Phase 1)
├── contracts/
│   └── api.md         (Phase 1)
└── quickstart.md      (Phase 1)
```

### Source code — A CRIAR (greenfield; nenhum codigo existe ainda)

```
backend/                          # Go module
├── cmd/api/main.go               # entrypoint HTTP
├── internal/
│   ├── domain/                   # entidades + maquinas de estado puras
│   │   ├── order.go              # state machine pedido (FR-008)
│   │   ├── commission.go         # state machine pagamento (FR-020)
│   │   └── money.go              # int64 centavos + RoundCommission (P-III)
│   ├── repository/               # pgx queries + mapper snake↔camel
│   ├── service/                  # apuracao idempotente, estorno, RBAC checks
│   ├── http/                     # handlers chi + middleware auth/RBAC
│   ├── auth/                     # JWT issue/verify (FR-024)
│   └── dto/                      # structs com json tags camelCase
├── migrations/                   # golang-migrate *.sql (schema + triggers + view)
└── go.mod

web/                              # React + Vite + TS
├── src/
│   ├── types/                    # DTO TS (camelCase) + zod schemas
│   ├── api/                      # react-query hooks (fetch + zod parse)
│   ├── pages/                    # vendors, orders, commissions, dashboards
│   └── components/               # charts (recharts), tables
└── package.json

e2e/                              # Playwright (quickstart §7 roundtrip)
```

> Paths sao a arvore ALVO proposta (projeto greenfield — `docs/` e `.git`
> sao a unica estrutura atual). create-tasks decompoe a criacao real.

---

## Convencoes de Borda

Feature multi-camada (DB ↔ backend Go ↔ frontend React). Fonte da verdade
de cada convencao:

| Camada | Case style | Validacao | Fonte da verdade |
|--------|------------|-----------|------------------|
| DB columns (PostgreSQL) | snake_case | CHECK constraints + FKs + triggers + migration | `backend/migrations/*.sql` |
| Backend DTO (Go) | camelCase (json tags) | validacao manual + CHECK no banco | `backend/internal/dto/*.go` |
| Frontend DTO (TS) | camelCase | zod parse no fetch | `web/src/types/*.ts` |
| API payload (req/resp) | camelCase | zod no frontend; manual no backend | `contracts/api.md` |
| Valores monetarios (payload) | inteiro de centavos (number) | range CHECK >= 0 | `contracts/api.md` + `data-model.md` |
| Percentuais (payload) | string decimal ("5.5000") | range [0,100] | `contracts/api.md` |
| URL query/path params | camelCase / kebab nos paths | router chi | `contracts/api.md` |

**Mapper layer (DB ↔ DTO)**:
- Backend: `backend/internal/repository/mapper.go` — snake_case (colunas) ↔
  camelCase (DTO). Mapeamento **manual explicito** (sem ORM auto-mapping —
  pgx, dec-032), garantindo controle do SQL financeiro sensivel.
- ORM auto-mapping: **NAO** (pgx + SQL explicito por decisao dec-032).

**Validacao Zod**:
- Borda: **response** no frontend (parse de toda resposta da API antes de
  usar) e **request** (valida antes de enviar). Backend re-valida no
  server-side (defesa em profundidade — UI nao e barreira, P-IV/FR-025).
- Schema compartilhado: schemas zod vivem em `web/src/types/`; nao ha
  pacote `shared-types` cross-language (Go nao consome zod). O contrato em
  `contracts/api.md` e a fonte unica que ambos os lados honram.

---

## Decisoes materiais (auditaveis)

Registradas no state.json como Decisao (dec-024..dec-032) — ver research.md:
representacao monetaria (dec-024), arredondamento (dec-025), auth JWT
(dec-026), versionamento de regra (dec-027), idempotencia (dec-028), saldo
liquido derivado (dec-029), trilha de auditoria (dec-030), LGPD anonimizacao
(dec-031), stack/libs (dec-032).

---

## Complexity Tracking

Nenhuma violacao de constitution exigindo justificativa. Arquitetura segue a
stack fixada com camadas convencionais (domain/repository/service/http);
nenhuma camada ou servico extra introduzido. Tabela vazia (sem desvios).

---

## Re-check de Constitution (pos Phase 1)

Apos modelo de dados + contratos:

| Principio | Re-check | Observacao pos-design |
|-----------|----------|------------------------|
| I | PASS | View `commission_net_balance` derivada (nao armazenada); triggers anti-UPDATE/DELETE confirmados no data-model; audit_trail transacional. Nenhuma complexidade injustificada. |
| II | PASS | Constraint `UNIQUE(order_id)` + `ON CONFLICT` cobre idempotencia no nivel do banco (alem do codigo). Regra por intervalo temporal resolve FR-012. |
| III | PASS | Auditoria de schema (SC-007) lista todas as colunas monetarias como BIGINT/NUMERIC. Arredondamento centralizado. |
| IV | PASS | Matriz RBAC explicita por endpoint; deny-by-default no middleware. |
| V | PASS | Inventario de 4 campos PII com finalidade; anonimizacao reconcilia FR-005 com P-I. |

Design **nao** introduziu complexidade nao justificada. Gate final: PASS.

---

## Artefatos

| Arquivo | Status |
|---------|--------|
| docs/specs/financial-dashboard/plan.md | Criado |
| docs/specs/financial-dashboard/research.md | Criado |
| docs/specs/financial-dashboard/data-model.md | Criado |
| docs/specs/financial-dashboard/contracts/api.md | Criado |
| docs/specs/financial-dashboard/quickstart.md | Criado |

**Constitution**: PASS (5/5, NON-NEGOTIABLE honrados). **NEEDS
CLARIFICATION restantes**: 0.

### Proximos Passos

1. `/checklist` — quality gate dos requisitos antes de implementar.
2. `/create-tasks` — decompor o plano em backlog executavel.
3. `/analyze` — validar consistencia spec↔plan↔tasks apos tasks.
