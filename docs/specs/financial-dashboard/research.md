# Research — Financial Dashboard

**Feature**: `financial-dashboard`
**Gerado por**: agente-00c, onda-005 (etapa plan, modo autonomo)
**Predecessor**: spec.md (Clarified); Constitution v1.0.0

> Phase 0 — resolucao de unknowns tecnicos. Cada decisao registra Decision /
> Rationale / Alternatives. Decisoes materiais tambem registradas como Decisao
> auditavel no state.json (dec-NNN citado).

---

## Decision 1 — Representacao monetaria (P-III NON-NEGOTIABLE)

**Decision**: Valores monetarios = inteiro de centavos (`int64` em Go,
`BIGINT` em PostgreSQL). Percentuais de comissao = `NUMERIC(7,4)` no
Postgres e `shopspring/decimal` no Go (ex: 5,5% armazenado como `5.5000`).

**Rationale**:
- `int64` em centavos e a forma mais simples e segura para valores
  monetarios em BRL (escala fixa de 2 casas). Soma/subtracao sao exatas;
  nenhum erro de arredondamento binario. Suporta ate ~92 quatrilhoes de
  centavos — folgado para o dominio.
- A multiplicacao `valor_centavos * percentual` produz fracao de centavo;
  o arredondamento e o unico ponto sensivel — ver Decision 2.
- Percentual NAO e dinheiro; precisa de casas decimais (5,5%). `NUMERIC(7,4)`
  no banco (range 0.0000–100.0000, FR-002) + `decimal.Decimal` no calculo
  evita float no fator multiplicativo.

**Alternatives considered**:
- `NUMERIC(15,2)` para dinheiro em vez de centavos: valido e tambem honra
  P-III, mas exige `shopspring/decimal` em todo trafego e mais cerimonia de
  serializacao. Centavos `int64` e mais leve para a borda da API (JSON
  number inteiro, sem string-encoding). Rejeitado por simplicidade; ambos
  satisfazem a constitution.
- `float64`: PROIBIDO por P-III. Nao considerado.

**Registro**: dec-024 (score 2 — suportado por P-III + briefing stack).

---

## Decision 2 — Politica de arredondamento (P-III)

**Decision**: Arredondamento half-up (round half away from zero) para o
centavo mais proximo, aplicado UMA unica vez no momento da apuracao da
comissao, sobre o produto `valor_pedido_centavos * percentual / 100`.
Documentado e centralizado numa unica funcao `money.RoundCommission`.

**Rationale**:
- P-III exige politica de arredondamento "unica, explicita e documentada"
  com ponto definido e testado. Half-up e a convencao comercial brasileira
  (alinhada a praticas contabeis de comissao).
- Calcular em `decimal.Decimal` (valor * percentual) e arredondar para
  `int64` centavos no fim evita acumulo de erro. O estorno (FR-027) usa o
  valor JA arredondado da comissao original (espelho exato, negativo) —
  zero divergencia de centavo.

**Alternatives considered**:
- Banker's rounding (half-to-even): reduz vies estatistico em grandes
  volumes, mas menos intuitivo para auditoria manual de comissao. Rejeitado
  — half-up e mais defensavel perante o vendedor.
- Truncamento: subpaga sistematicamente o vendedor. Rejeitado.

**Registro**: dec-025 (score 2).

---

## Decision 3 — Mecanismo de autenticacao (FR-024, TODO(AUTH_MECHANISM))

**Decision**: JWT stateless (HS256 com secret de ambiente no MVP; caminho
para RS256 pos-MVP) com refresh token de vida curta. Claims minimas:
`sub` (user_id), `role` (gestor|vendedor|financeiro), `vendor_id`
(nullable, so para papel vendedor), `exp`.

**Rationale**:
- FR-024 deixa a escolha para plan; ambas (JWT vs sessao) sao compativeis.
- JWT stateless simplifica o backend Go (sem store de sessao no MVP) e
  carrega `role` + `vendor_id` necessarios ao RBAC (P-IV) e ao escopo de
  dados do Vendedor (FR-017, SC-005).
- Deny-by-default (P-IV, FR-026): middleware exige token valido em TODA
  rota exceto `/auth/login`; ausencia de claim de papel autorizado = 403.

**Alternatives considered**:
- Sessao server-side (cookie + store): permite revogacao imediata, mas
  adiciona store stateful. Para POC sem requisito de revogacao instantanea,
  overkill. Pos-MVP pode migrar. Rejeitado para o MVP.
- OAuth2/OIDC externo: fora de escopo (sem IdP no briefing). Rejeitado.

**Nota de seguranca**: secret JWT via variavel de ambiente, NUNCA commitado.
`vendor_id` no token e fonte de verdade do escopo, mas a checagem de
ownership e SEMPRE re-validada no backend contra o recurso (defesa em
profundidade — token nao e barreira unica).

**Registro**: dec-026 (score 2).

---

## Decision 4 — Versionamento de Regra de Comissao (P-II, FR-003, FR-012)

**Decision**: Tabela `commission_rules` append-only, uma linha por versao,
com `(vendor_id, version, valid_from, valid_to, percentage)`. `valid_to`
NULL = versao vigente. Selecao da regra na apuracao usa a `data_do_pedido`:
`valid_from <= order_date AND (valid_to IS NULL OR order_date < valid_to)`.

**Rationale**:
- FR-012 + dec-020: percentual vigente e o da DATA DO PEDIDO, nao da
  apuracao. Modelar como intervalos temporais (`valid_from`/`valid_to`)
  resolve isso com uma query deterministica.
- Editar percentual (FR-003) = fechar a versao atual (`valid_to = now`) +
  inserir nova versao. Registros antigos nunca mudam (P-I, P-II).
- A Comissao apurada referencia `commission_rule_id` (FK imutavel),
  garantindo reconstrucao do calculo mesmo apos novas mudancas (P-I).

**Alternatives considered**:
- Coluna `percentage` mutavel em `vendors` + tabela de historico separada:
  duplica fonte de verdade; risco de divergencia. Rejeitado.
- Event sourcing puro: overkill para POC; a tabela versionada ja da
  reconstrucao temporal suficiente. Rejeitado.

**Registro**: dec-027 (score 2).

---

## Decision 5 — Idempotencia da apuracao (P-II, FR-014, SC-003)

**Decision**: Constraint de unicidade `UNIQUE (order_id)` em `commissions`
(uma comissao por pedido pago, no maximo). Apuracao por periodo roda em
transacao: para cada pedido `pago` no periodo sem comissao existente,
`INSERT ... ON CONFLICT (order_id) DO NOTHING`. Reexecutar nao duplica nem
altera (P-II).

**Rationale**:
- A unicidade por `order_id` e o invariante natural: comissao deriva 1:1 de
  um pedido pago. `ON CONFLICT DO NOTHING` torna a apuracao idempotente no
  nivel do banco (constraint, nao so codigo — Padroes Tecnicos da
  constitution exige integridade relacional via constraints).
- Crash a meio (Edge Case): a transacao por pedido + constraint garante que
  o retry so insere o que faltava. SC-003 satisfeito.

**Alternatives considered**:
- Chave de idempotencia por `(vendor_id, period)`: nao captura granularidade
  por pedido; um pedido pago tardiamente no mesmo periodo nao seria apurado.
  Rejeitado.
- Dedup so em codigo (sem constraint): viola "integridade via constraints".
  Rejeitado.

**Registro**: dec-028 (score 2).

---

## Decision 6 — Saldo liquido de comissao derivado, nao armazenado (FR-029)

**Decision**: O valor liquido de uma comissao = `commission.value +
SUM(reversals.value)` (estornos sao negativos), computado em query/view, NAO
persistido como campo editavel. `commissions` e `commission_reversals` sao
ambas append-only e imutaveis pos-insert.

**Rationale**:
- FR-029 crava: registros originais MUST NOT ser alterados; saldo liquido e
  derivado computacionalmente. Uma VIEW `commission_net_balance` materializa
  isso de forma reconcomputavel (FR-018, P-I).
- Imutabilidade reforcada por trigger `BEFORE UPDATE/DELETE` que rejeita
  (defesa em profundidade alem de disciplina de codigo).

**Alternatives considered**:
- Campo `net_value` atualizado por trigger: viola imutabilidade (registro
  passa a ter campo mutavel). Rejeitado por FR-029.

**Registro**: dec-029 (score 2).

---

## Decision 7 — Trilha de auditoria (P-I, FR-009, FR-021)

**Decision**: Tabela unica `audit_trail` append-only, polimorfica por
`(entity_type, entity_id)`, com `actor_user_id`, `action`, `from_state`,
`to_state`, `reason` (obrigatorio em reversoes), `occurred_at` (UTC).
Trigger rejeita UPDATE/DELETE. Toda transicao de pedido e de pagamento de
comissao grava uma linha na MESMA transacao da mudanca de estado.

**Rationale**:
- P-I exige trilha append-only com ator/timestamp/motivo para transicoes de
  pedido (FR-009) e pagamento (FR-021). Tabela unica polimorfica simplifica
  consulta cross-entidade e e suficiente para o volume POC.
- Gravar na mesma transacao da transicao garante atomicidade (sem transicao
  sem trilha — SC-006).

**Alternatives considered**:
- Trilha por-entidade (tabelas separadas): mais normalizado mas mais
  boilerplate; sem ganho para POC. Rejeitado.
- Log estruturado em arquivo como unica trilha: nao transacional, nao
  rastreavel relacionalmente. Rejeitado (logs sao complementares — Padroes
  Tecnicos pede observabilidade ALEM da trilha persistida).

**Registro**: dec-030 (score 2).

---

## Decision 8 — LGPD: exclusao com preservacao financeira (P-V, FR-005)

**Decision**: Soft-anonymization. Exclusao a pedido do titular substitui PII
(`name`, `email`) por tokens (`[REDACTED]` + hash irreversivel para
deduplicacao) e marca `anonymized_at`, preservando `id` e todos os
registros financeiros (pedidos, comissoes) que referenciam o vendedor por
FK. P-I prevalece: registros financeiros imutaveis sobrevivem anonimizados.

**Rationale**:
- FR-005 + P-V: direito de exclusao do titular, MAS P-I prevalece sobre
  exclusao irrestrita — auditoria financeira nao pode quebrar. Anonimizar a
  PII (nao deletar a linha) reconcilia ambos.
- Minimizacao (P-V, FR-004): so 4 campos pessoais coletados (nome, email,
  id, percentual); cada um mapeado a finalidade no data-model (SC-008).

**Alternatives considered**:
- Hard delete com cascade: quebraria FKs de comissoes e a auditoria (P-I).
  Rejeitado.
- Criptografia de PII at-rest com key destruction: spec exclui cripto alem
  do nativo do Postgres no MVP. Rejeitado para o MVP.

**Registro**: dec-031 (score 2).

---

## Decision 9 — Stack concreta e bibliotecas

**Decision**:
- **Backend**: Go 1.22+, `net/http` + `chi` router, `pgx/v5` para Postgres,
  `golang-migrate` para migrations, `shopspring/decimal` para percentuais,
  `golang-jwt/jwt/v5` para tokens, `slog` para logs estruturados.
- **Frontend**: React 18 + TypeScript + Vite, `@tanstack/react-query` para
  data-fetching, `react-router`, `recharts` para os graficos do dashboard,
  `zod` para validacao de payload nas bordas.
- **Banco**: PostgreSQL 16.
- **Testes**: Go `testing` + `testify`; frontend `vitest` + Playwright para
  o roundtrip E2E (quickstart §Roundtrip).

**Rationale**: stack fixada pela constitution (Go + React + Postgres);
bibliotecas sao mainstream, com baixa superficie de deps e bom suporte a
decimal/JWT/migrations. `zod` em ambas as bordas suporta a convencao de
borda (camelCase) declarada no plan.

**Alternatives considered**: GORM (ORM) vs pgx (driver+queries): pgx
escolhido para controle explicito do SQL (critico para a query de regra
vigente e o `ON CONFLICT` de idempotencia). Rejeitado GORM por ocultar SQL
sensivel ao dominio financeiro.

**Registro**: dec-032 (score 2).

---

## NEEDS CLARIFICATION restantes

Nenhum. Todos os unknowns tecnicos foram resolvidos com defaults sensatos
suportados por spec + constitution + briefing. As decisoes materiais estao
registradas como Decisao auditavel (dec-024..dec-032).
