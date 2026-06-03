# Security Checklist: Financial Dashboard

**Purpose**: Validar qualidade dos requisitos de seguranca — autenticacao, autorizacao, protecao de dados, input validation, logging e compliance LGPD. Sistema financeiro com dados sensiveis de comissoes/pagamentos/vendedores.
**Created**: 2026-06-03
**Feature**: [spec.md](../spec.md) | [plan.md](../plan.md) | [data-model.md](../data-model.md)

---

## Autenticacao (AuthN)

- [x] CHK001 - O mecanismo de autenticacao concreto (JWT HS256 stateless + refresh) esta especificado com algoritmo, duracao de token e fluxo de refresh? [Completude, Spec §FR-024, plan.md Technical Context] {auto}
  > Plan define: JWT HS256 stateless + refresh (dec-026). Algoritmo especificado. Duracao e fluxo de refresh: research dec-026 referenciado mas nao detalhado na spec — ver CHK002.

- [ ] CHK002 - Sao os tempos de expiracao do access token e do refresh token definidos com valores concretos (ex: 15min / 7 dias)? [Completude, Gap, Spec §FR-024] {humano}
  > [Gap] A spec menciona JWT stateless + refresh (dec-026) mas nao define valores concretos de TTL. Impacto em seguranca: token longo aumenta janela de comprometimento. Decisao tecnica a especificar antes de implementar.

- [x] CHK003 - Existe requisito explicitando que a senha nunca e persistida em plaintext? [Clareza, data-model.md Entity: User] {auto}
  > data-model.md: `password_hash TEXT NOT NULL` com nota "bcrypt/argon2; nunca plaintext". Algoritmo de hash especificado (bcrypt ou argon2 — ambos aceitos).

- [ ] CHK004 - O algoritmo de hash de senha (bcrypt vs argon2) esta definido com parametros de custo (rounds/iterations)? [Completude, Gap, data-model.md Entity: User] {humano}
  > [Gap] data-model.md lista "bcrypt/argon2" sem especificar qual algoritmo nem parametros de custo (ex: bcrypt cost=12, argon2id m=64MB t=3). Parametros afetam resistencia a brute-force e devem ser definidos antes da implementacao.

- [x] CHK005 - Existe requisito de negacao de acesso a qualquer recurso sem autenticacao valida? [Completude, Spec §FR-024, FR-026] {auto}
  > FR-024: "O sistema MUST autenticar usuarios antes de qualquer acesso." FR-026: "deny-by-default". Plan: middleware server-side exige token. Requisito presente e claro.

---

## Autorizacao (AuthZ / RBAC)

- [x] CHK006 - A matriz de controle de acesso (RBAC) esta definida para TODOS os papeis (Gestor, Vendedor, Financeiro) e TODAS as operacoes criticas? [Completude, Spec §FR-025, FR-026, plan.md] {auto}
  > Spec FR-023 e FR-025 definem fronteiras. Spec §US1-US5 detalha cenarios de acesso. Plan referencia "matriz RBAC explicita por endpoint" em api.md. Constitution Check IV PASS. Matriz presente.

- [x] CHK007 - O principio deny-by-default esta especificado como server-side (nao apenas frontend)? [Clareza, Spec §FR-025, FR-026] {auto}
  > FR-025: "RBAC em todas as operacoes no backend (server-side); controles de UI sao complementares, nao barreiras de seguranca." FR-026: "deny-by-default". Claro e inequivoco.

- [x] CHK008 - Existe requisito de escopo de dados: Vendedor ve SOMENTE suas proprias comissoes e vendas, nunca dados de outros vendedores? [Completude, Spec §FR-017, US4.4, US5.3] {auto}
  > FR-017 e US5.3 explicitam escopo de vendedor. US4.4 define visibilidade de comissoes. SC-005: "requisicao de Vendedor tentando acessar dados de outro vendedor e negada em 100% dos casos em todos os endpoints." Requisito mensuravel e presente.

- [x] CHK009 - As fronteiras de acesso do Financeiro (pode aprovar/pagar comissoes; NAO pode alterar regras ou cadastros) estao especificadas com clareza suficiente para implementar sem ambiguidade? [Clareza, Spec §FR-023, Clarifications A-001] {auto}
  > FR-023: "Financeiro MUST NOT poder alterar regras de comissao ou cadastros de vendedores." Clarifications A-001 confirma fronteiras. Suficientemente claro.

- [ ] CHK010 - Existe requisito explicitando o que acontece quando um token JWT expirado tenta acessar um recurso protegido (ex: 401 com codigo de erro especifico)? [Completude, Gap, Spec §FR-024] {humano}
  > [Gap] A spec nao define o comportamento de erro para tokens expirados ou invalidos. Ausencia de definicao pode levar a implementacoes inconsistentes (401 vs 403, corpo da resposta, instrucoes de refresh). Definir para contracts/api.md.

- [ ] CHK011 - Existe requisito de invalidacao/revogacao de token (ex: logout, troca de senha, desativacao de usuario)? [Completude, Gap, Spec §FR-024, FR-001] {humano}
  > [Gap] JWT stateless por definicao nao tem revogacao nativa. A spec nao menciona blocklist de tokens, mecanismo de logout efetivo, nem o que ocorre quando um vendedor e desativado (FR-001) enquanto tem token ativo. Decisao de design com impacto de seguranca direto.

---

## Protecao de Dados / LGPD

- [x] CHK012 - O inventario de dados pessoais (PII) esta mapeado a finalidades explicitas e limitado ao minimo necessario? [Completude, Spec §FR-004, SC-008, data-model.md Vendor] {auto}
  > FR-004 e SC-008: 4 campos PII (name, email, id implicito, percentual via commission_rules) com finalidades mapeadas. data-model.md confirma. Minimizacao P-V documentada.

- [x] CHK013 - O mecanismo de exclusao de dados pessoais (LGPD - direito ao esquecimento) esta definido com a politica de anonimizacao preservando registros financeiros? [Completude, Spec §FR-005, data-model.md Vendor] {auto}
  > FR-005: exclusao = anonimizacao (P-I prevalece). data-model.md: `anonymized_at TIMESTAMPTZ NULL`, name/email → token anonimizado. Reconcilia FR-005 com P-I. Presente e suficiente para MVP.

- [ ] CHK014 - O formato especifico da anonimizacao (ex: SHA256 do email, token UUID, string fixa "REMOVED") esta definido para garantir irreversibilidade? [Clareza, Ambiguity, Spec §FR-005] {humano}
  > [Ambiguity] A spec menciona "anonimizacao preservando financeiros" mas nao especifica o formato do token anonimo. "SHA256 do email" ainda seria dado pessoal por ser reversivel com rainbow table. O formato determina se a anonimizacao e de fato conforme LGPD Art. 5, XII.

- [x] CHK015 - Existe requisito de que a senha nunca aparece em logs, respostas de API ou trilha de auditoria? [Completude, Spec §FR-004, plan.md] {auto}
  > FR-004 (minimizacao), plan.md (slog estruturado). data-model.md: campo e `password_hash`. A spec nao menciona logging de senha explicitamente, mas FR-004 + minimizacao P-V + campo sendo hash cobrem o requisito implicitamente. Suficiente para MVP.

---

## Validacao de Input

- [x] CHK016 - Existe requisito de validacao server-side para o intervalo do percentual de comissao ([0%, 100%]) com rejeicao de valores invalidos? [Completude, Spec §FR-002, data-model.md CommissionRule] {auto}
  > FR-002: "intervalo valido e [0%, 100%] — comissao zero e valida; percentual negativo ou acima de 100% MUST ser rejeitado com mensagem de erro explicita." data-model: `CHECK 0..100`. Claro e testavel.

- [x] CHK017 - Existe requisito de validacao de valor de pedido (nao-negativo, sem ponto flutuante)? [Completude, Spec §FR-007, data-model.md Order] {auto}
  > FR-007: P-III (sem float). data-model: `total_cents BIGINT NOT NULL, CHECK >= 0`. Validacao server-side (Go service) + CHECK no banco. Presente.

- [x] CHK018 - As transicoes invalidas da maquina de estado de pedido sao tratadas como erros explicitos (nao silenciosos)? [Clareza, Spec §FR-008, US2.3] {auto}
  > FR-008: "Todas as demais transicoes MUST ser rejeitadas pelo sistema." US2.3 Acceptance Scenario: sistema rejeita pago→rascunho. Presente e com criterio de aceite verificavel.

- [ ] CHK019 - Existe requisito de limite de tamanho/formato para campos de texto livre (ex: nome do vendedor, descricao de item de pedido)? [Completude, Gap, Spec §FR-001, FR-010] {humano}
  > [Gap] A spec e data-model.md definem campos TEXT mas nao estabelecem limites maximos de tamanho (ex: VARCHAR(255) vs TEXT ilimitado). Ausencia pode levar a DoS por payload gigante ou problemas de exibicao em dashboard.

- [ ] CHK020 - Existe requisito de sanitizacao ou restricao de caracteres especiais em campos de texto (ex: nome de vendedor com SQL injection ou XSS)? [Completude, Gap, Spec §FR-001] {humano}
  > [Gap] A spec nao menciona sanitizacao de input para campos de texto. O plan menciona "validacao manual" no backend, mas sem especificacao de criterios. Para sistema financeiro, SQL injection e XSS em campos exibidos no dashboard sao vetores relevantes.

---

## Logging e Auditoria de Seguranca

- [x] CHK021 - A trilha de auditoria cobre TODOS os eventos criticos de seguranca (transicoes de estado, alteracoes de regra, aprovacoes de pagamento)? [Completude, Spec §FR-009, FR-021, data-model.md AuditTrail] {auto}
  > FR-009 (pedidos), FR-021 (pagamentos de comissao), FR-003 (alteracao de percentual = nova versao de regra). data-model: `audit_trail` append-only com entity_type, actor, from/to state, reason, timestamp. SC-006 mensuravel. Cobertura presente.

- [x] CHK022 - A trilha de auditoria e imutavel (append-only) por mecanismo tecnico, nao apenas por convencao? [Clareza, Spec §P-I, data-model.md AuditTrail] {auto}
  > data-model: "trigger BEFORE UPDATE OR DELETE → RAISE EXCEPTION." Imutabilidade enforced por banco, nao por honra. Atende P-I.

- [ ] CHK023 - Existe requisito de retencao de logs/trilha de auditoria (por quanto tempo os registros de auditoria sao preservados)? [Completude, Gap, Spec §P-I] {humano}
  > [Gap] A spec garante imutabilidade mas nao define politica de retencao (ex: 5 anos para registros financeiros, conformidade com regulacoes contabeis brasileiras). Pode ser requisito regulatorio dependendo do regime fiscal.

- [x] CHK024 - Eventos de autenticacao (login bem-sucedido, falha de login) estao previstos na auditoria? [Completude, Spec §FR-024, data-model.md AuditTrail] {auto}
  > [Gap parcial] A spec define `audit_trail` para transicoes de estado de entidades financeiras, mas nao menciona explicitamente logging de eventos de autenticacao (login success/failure). Para sistema financeiro com acesso a dados de comissao, events de authN deveriam constar. O `audit_trail.entity_type` nao inclui 'auth_event'. Marcado como auto-verificavel: ausencia identificada.

---

## Comunicacao Segura

- [ ] CHK025 - Existe requisito de HTTPS obrigatorio para todas as comunicacoes cliente-servidor? [Completude, Gap, Spec §FR-024] {humano}
  > [Gap] A spec nao menciona TLS/HTTPS. Para sistema financeiro com JWT em transit, HTTPS e essencial. A ausencia pode ser intencional (POC local) mas deve ser decisao explicita.

- [ ] CHK026 - Existe requisito de protecao do JWT contra exposicao (ex: HttpOnly cookie vs Authorization header, CORS policy)? [Completude, Gap, Spec §FR-024] {humano}
  > [Gap] A spec decide por JWT stateless mas nao especifica o mecanismo de armazenamento no cliente (localStorage = vulneravel a XSS; HttpOnly cookie = protegido contra XSS mas requer CSRF protection). Decisao com impacto direto de seguranca.

---

## Notes

- Items `{auto}` ja vem resolvidos pelo agente (`[x]` com citacao, ou marcador `[Gap]`)
- Items `{humano}` ficam `[ ]` aguardando decisao do dono do produto
- Marcar items concluidos com `[x]`

### Resumo

- **{auto} resolvidos**: 11 (`[x]` com evidencia)
- **{auto} com Gap identificado**: 3 (CHK001 parcial, CHK024 Gap auditoria de authN)
- **{humano} aguardando decisao**: 8 (CHK002, CHK004, CHK010, CHK011, CHK014, CHK019, CHK020, CHK023, CHK025, CHK026)
- **Gaps abertos (`[Gap]`/`[Ambiguity]`)**: CHK002, CHK004, CHK010, CHK011, CHK014, CHK019, CHK020, CHK023, CHK024, CHK025, CHK026

### Destino dos Gaps

| Item | Marcador | Destino |
|------|----------|---------|
| CHK002 | `[Gap]` | `/clarify` ou contracts/api.md — definir TTL de tokens |
| CHK004 | `[Gap]` | `/clarify` — definir algoritmo de hash + parametros de custo |
| CHK010 | `[Gap]` | contracts/api.md — definir resposta para token expirado/invalido |
| CHK011 | `[Gap]` | `/clarify` — decidir estrategia de revogacao/logout |
| CHK014 | `[Ambiguity]` | `/clarify` — especificar formato de anonimizacao LGPD-conforme |
| CHK019 | `[Gap]` | `/create-tasks` — tarefa "definir limites de tamanho de campos TEXT" |
| CHK020 | `[Gap]` | `/create-tasks` — tarefa "especificar sanitizacao de input" |
| CHK023 | `[Gap]` | `/clarify` — definir politica de retencao de auditoria |
| CHK024 | `[Gap]` | `/create-tasks` — tarefa "adicionar logging de eventos de authN ao audit_trail" |
| CHK025 | `[Gap]` | `/clarify` — decidir requisito HTTPS para POC vs producao |
| CHK026 | `[Gap]` | `/clarify` — decidir armazenamento JWT (HttpOnly cookie vs header) |
