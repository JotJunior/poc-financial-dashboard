# API Checklist: Financial Dashboard

**Purpose**: Validar qualidade dos requisitos de contratos de API — endpoints, error handling, versionamento, rate limiting, observabilidade, contratos request/response e consistencia entre spec e plan.
**Created**: 2026-06-03
**Feature**: [spec.md](../spec.md) | [plan.md](../plan.md) | [contracts/api.md](../contracts/api.md)

---

## Contratos de Endpoint

- [x] CHK027 - Os endpoints CRUD de vendedores estao especificados com metodos HTTP, paths e papeis autorizados? [Completude, Spec §FR-001, plan contracts/api.md] {auto}
  > Plan referencia `contracts/api.md` como fonte de contratos. FR-001 define operacoes CRUD. A matriz RBAC por endpoint e mencionada no plan (Constitution Check IV). Contratos em api.md — artefato criado na onda-005.

- [x] CHK028 - O contrato do endpoint de apuracao de comissao (FR-015) especifica o formato do parametro de periodo (mes/ano) e a resposta esperada? [Completude, Spec §FR-015, data-model.md Commission] {auto}
  > FR-015: "periodo mensal, sob demanda." data-model: `period TEXT "YYYY-MM"`. Plan referencia api.md para contratos. Formato "YYYY-MM" definido no data-model. Suficiente para implementar.

- [x] CHK029 - O endpoint de transicao de estado de pedido especifica quais transicoes sao validas e quais retornam erro? [Completude, Spec §FR-008, data-model.md Order state machine] {auto}
  > FR-008: 4 transicoes validas definidas explicitamente; "Todas as demais MUST ser rejeitadas." State machine em data-model com Mermaid diagram. Criterio de rejeicao claro.

- [x] CHK030 - O endpoint de aprovacao/pagamento de comissao especifica o campo `reason` como obrigatorio em reversoes? [Completude, Spec §FR-020, FR-021, data-model.md Commission state machine] {auto}
  > FR-020: "aprovado → pendente (Financeiro reverte aprovacao — com motivo obrigatorio)". FR-021: "reason (obrigatorio em reversoes)". data-model: `audit_trail.reason TEXT NULL — obrigatorio em reversoes`. Presente.

- [ ] CHK031 - Os contratos de API definem o formato de paginacao para listas (ex: page/limit, cursor, total_count) para endpoints que podem retornar grandes volumes? [Completude, Gap, Spec §FR-016, FR-022] {humano}
  > [Gap] FR-016 (dashboard Gestor), FR-022 (lista de comissoes por status) e FR-001 (lista de vendedores) podem retornar volumes grandes. A spec nao define paginacao, cursor ou tamanho maximo de pagina. Ausencia impacta performance e usabilidade do frontend.

- [ ] CHK032 - Os contratos de resposta de dashboard definem o schema exato de campos retornados (total_vendas, total_comissoes, ranking_vendedores)? [Completude, Gap, Spec §FR-016] {humano}
  > [Gap] FR-016 enumera os campos do dashboard ("total de vendas, total de comissoes apuradas e lista de vendedores com volumes"), mas nao define o schema JSON exato (nomes de campos, tipos, unidade monetaria — centavos ou reais?). A ausencia pode levar a inconsistencia entre backend e frontend durante implementacao.

---

## Error Handling

- [ ] CHK033 - Os codigos de status HTTP para cada cenario de erro estao mapeados nos contratos? [Completude, Gap, Spec §FR-008, FR-020, FR-025] {humano}
  > [Gap] A spec define as rejeicoes de negocio (transicao invalida, RBAC deny, validacao) mas nao especifica os codigos HTTP correspondentes (400 vs 422 para input invalido, 401 vs 403 para RBAC). Sem padrao definido, implementacoes distintas podem surgir.

- [ ] CHK034 - O formato do body de erro esta padronizado (ex: `{"error": {"code": "...", "message": "..."}}`) para todos os endpoints? [Completude, Gap, Spec §FR-001..FR-029] {humano}
  > [Gap] A spec menciona "mensagem de erro explicita" (FR-002) mas nao define um schema padrao de resposta de erro. Frontend precisa de schema consistente para exibir mensagens ao usuario.

- [x] CHK035 - Existe requisito de que erros de autorizacao (RBAC deny) nao vazam informacao sobre recursos existentes (ex: 403 vs 404 para recurso de outro usuario)? [Completude, Spec §FR-026, SC-005] {auto}
  > FR-026: "deny-by-default." SC-005: "requisicao de Vendedor tentando acessar dados de outro vendedor e negada em 100% dos casos." A spec nao explicita 403 vs 404, mas a exigencia de deny-by-default + RBAC server-side implica que o requisito existe. [Ambiguity] A escolha entre 403 (recurso existe, sem acesso) vs 404 (recurso inexistente do ponto de vista do caller) e uma decisao de seguranca que deveria ser explicita — 404 e mais seguro por nao revelar existencia do recurso.

---

## Rate Limiting e Protecao

- [ ] CHK036 - Existe requisito de rate limiting ou throttling para endpoints criticos (ex: login, apuracao de comissao)? [Completude, Gap, Spec §FR-024, FR-015] {humano}
  > [Gap] A spec nao menciona rate limiting. Para sistema financeiro, endpoints de autenticacao sem rate limiting sao vulneraveis a brute-force (CHK002/CHK004 relacionados). Apuracao sem throttling pode ser disparada indefinidamente.

- [ ] CHK037 - Existe requisito de protecao contra requisicoes duplicadas em operacoes financeiras criticas (idempotency keys para POST de pedido, POST de pagamento)? [Completude, Gap, Spec §FR-014] {humano}
  > [Gap] FR-014 define idempotencia de APURACAO de comissao (via UNIQUE order_id). Mas nao ha requisito de idempotency key para criacao de pedido (POST /orders) — duas requisicoes identicas podem criar dois pedidos distintos. Em sistema financeiro, isso e um risco de duplicidade.

---

## Versionamento e Compatibilidade

- [ ] CHK038 - Existe estrategia de versionamento de API definida (ex: /v1/ no path, header Accept-Version)? [Completude, Gap, Spec geral] {humano}
  > [Gap] A spec e plan nao mencionam versionamento de API. Para POC, pode ser aceitavel, mas a ausencia de estrategia definida (mesmo que "sem versionamento no MVP") dificulta evolucao futura. Decisao explicita necessaria.

---

## Consistencia de Representacao

- [x] CHK039 - Todos os valores monetarios sao representados de forma consistente na API (inteiro de centavos, nao float)? [Completude, Spec §FR-007, plan.md Convencoes de Borda] {auto}
  > Plan Convencoes de Borda: "Valores monetarios (payload): inteiro de centavos (number)." data-model: BIGINT centavos em todas as colunas monetarias. Consistente com P-III. Presente e aplicavel aos contratos.

- [x] CHK040 - Os percentuais sao representados como string decimal ("5.5000") na API para evitar perda de precisao em JSON? [Completude, plan.md Convencoes de Borda, Spec §FR-002] {auto}
  > Plan Convencoes de Borda: "Percentuais (payload): string decimal ('5.5000')." Consistente com NUMERIC(7,4) no banco. Previne perda de precisao em JSON number. Presente.

- [x] CHK041 - Os timestamps estao definidos como UTC em toda a API? [Completude, Spec Edge Cases, data-model.md] {auto}
  > Spec Edge Cases: "Todas as operacoes usam UTC como referencia interna e de persistencia." data-model: todos os campos de tempo sao `TIMESTAMPTZ NOT NULL DEFAULT now()`. Consistente.

- [x] CHK042 - O case style dos campos de payload esta padronizado (camelCase) e documentado? [Clareza, plan.md Convencoes de Borda] {auto}
  > Plan Convencoes de Borda: "API payload (req/resp): camelCase" com fonte de verdade em contracts/api.md. Mapper layer documentado. Presente.

---

## Observabilidade

- [ ] CHK043 - Existe requisito de logging estruturado para cada requisicao de API (metodo, path, status, latencia, actor)? [Completude, Gap, Spec §P-I, plan.md] {humano}
  > [Gap] Plan menciona "slog (estruturado)" mas a spec nao define o que deve ser logado por requisicao. Para sistema financeiro com P-I (auditabilidade), o nivel de detalhe de logging por request (incluindo actor/user_id) deveria ser especificado.

- [ ] CHK044 - Existe requisito de health check endpoint para monitoramento da aplicacao? [Completude, Gap, Spec geral] {humano}
  > [Gap] A spec nao menciona health check (`/health`, `/ready`). Para qualquer sistema em producao (mesmo POC), um endpoint de health check e pratica basica. Decisao de incluir ou nao deveria ser explicita.

---

## Notes

- Items `{auto}` ja vem resolvidos pelo agente (`[x]` com citacao, ou marcador `[Gap]`)
- Items `{humano}` ficam `[ ]` aguardando decisao do dono do produto
- Marcar items concluidos com `[x]`

### Resumo

- **{auto} resolvidos**: 8 (`[x]` com evidencia)
- **{humano} aguardando decisao**: 10 (CHK031..CHK038, CHK043, CHK044)
- **Gaps abertos (`[Gap]`/`[Ambiguity]`)**: CHK031, CHK032, CHK033, CHK034, CHK035(Ambiguity), CHK036, CHK037, CHK038, CHK043, CHK044

### Destino dos Gaps

| Item | Marcador | Destino |
|------|----------|---------|
| CHK031 | `[Gap]` | `/create-tasks` — tarefa "definir paginacao nos contratos de API" |
| CHK032 | `[Gap]` | `/create-tasks` — tarefa "detalhar schema JSON dos endpoints de dashboard" |
| CHK033 | `[Gap]` | `/create-tasks` — tarefa "mapear codigos HTTP por cenario de erro em api.md" |
| CHK034 | `[Gap]` | `/create-tasks` — tarefa "padronizar schema de body de erro" |
| CHK035 | `[Ambiguity]` | `/clarify` — decidir 403 vs 404 para recursos de outro usuario |
| CHK036 | `[Gap]` | `/clarify` — decidir se rate limiting e requisito do MVP |
| CHK037 | `[Gap]` | `/clarify` — decidir idempotency keys para criacao de pedido |
| CHK038 | `[Gap]` | `/clarify` — definir estrategia de versionamento de API (mesmo que "sem versao no MVP") |
| CHK043 | `[Gap]` | `/create-tasks` — tarefa "definir campos obrigatorios de logging por request" |
| CHK044 | `[Gap]` | `/create-tasks` — tarefa "adicionar health check endpoint ao plan" |
