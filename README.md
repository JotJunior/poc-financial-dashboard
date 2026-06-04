# Financial Dashboard

Sistema de **cadastro de vendedores, registro de pedidos, cálculo de comissões,
controle de pagamentos e dashboards de performance de vendas**.

> ⚠️ **Este projeto é um experimento.** Todo o conteúdo deste repositório —
> briefing, especificação, plano técnico, código (backend Go + frontend React +
> migrations PostgreSQL), testes e documentação — foi **gerado de forma autônoma**
> pelo **[cstk](https://github.com/) + agente-00c**, um orquestrador de pipeline
> SDD (Spec-Driven Development) executado sem intervenção manual de codificação.
> É uma **prova de conceito**, não software pronto para produção.

---

## 🤖 Como este projeto foi construído (o experimento)

A partir de uma única frase de descrição e uma pasta vazia, o **agente-00c**
conduziu, em **20 "ondas"** autônomas, a pipeline completa de SDD:

```
briefing → constitution → specify → clarify → plan → checklist
        → create-tasks → execute-task → review-task → review-features
```

- **20 ondas** de execução, com seleção de modelo por fase (haiku/sonnet/opus).
- **81 decisões auditáveis** registradas (cada escolha material com contexto,
  opções, justificativa).
- **37 tasks / 219 subtarefas** (100% concluídas), backlog rastreável.
- **0 bloqueios humanos** genuínos durante a construção.
- Cada onda fechada com commit local + ingestão em base de conhecimento.

A trilha completa da execução autônoma fica em:

- `.claude/agente-00c-report.md` — relatório consolidado.
- `.claude/agente-00c-state/state.json` — estado/decisões da execução.
- `docs/` — briefing, constituição, spec, plano, contratos, checklists, tasks.

> As duas únicas intervenções humanas relevantes pós-geração foram a operação de
> um navegador real (via Playwright MCP) para **conferir a UI**, o que revelou e
> levou à correção de 3 bugs (roteamento pós-login/sessão e o formato de
> `appliedPercentage` nas comissões) — exatamente o tipo de defeito que só aparece
> exercitando a interface de verdade.

### 🎥 Gravações de tela do desenvolvimento

Toda a execução autônoma foi gravada (screencast), em ordem cronológica:

| Parte | Conteúdo | Vídeo |
|-------|----------|-------|
| **0** | Comando inicial · briefing · constitution · plan · checklist · create-tasks | https://youtu.be/LM7x5Gwq9qU |
| **1** | Desenvolvimento — ondas 8 a 10 | https://youtu.be/7eGQErrGea8 |
| **2** | Desenvolvimento — ondas 11 a 13 | https://youtu.be/aDvnIBi8EUo |
| **3** | Desenvolvimento — onda 14 | https://youtu.be/YOcTWU3PzkI |
| **4** | Desenvolvimento — ondas 15 a 17 | https://youtu.be/6SXcYtPpgOM |
| **5** | Desenvolvimento — onda 17 + idas e vindas dos testes headless do Playwright | https://youtu.be/saG5OYiQ6KY |
| **6** | Finalização — testes E2E + Playwright headed (no navegador) | https://youtu.be/QQeiQgqkAq0 |

---

## 🧱 Stack

| Camada | Tecnologias |
|--------|-------------|
| **Backend** | Go 1.26 · [chi](https://github.com/go-chi/chi) · [pgx](https://github.com/jackc/pgx) · [shopspring/decimal](https://github.com/shopspring/decimal) · [golang-jwt](https://github.com/golang-jwt/jwt) · [golang-migrate](https://github.com/golang-migrate/migrate) |
| **Frontend** | React 19 · Vite · TypeScript · TanStack Query · Zod |
| **Banco** | PostgreSQL 16 (`NUMERIC(7,4)` + triggers de imutabilidade) |
| **Testes** | `go test` (unit + integração) · Playwright (E2E API-driven + browser-driven) |
| **Infra** | Docker Compose · Dockerfiles multi-stage · nginx (perfil de produção) |

### Princípios de governança (constituição do projeto)

A constituição (`docs/constitution.md`) fixou 5 invariantes **verificados em runtime**:

1. **Auditabilidade financeira total** — trilha `audit_trail` append-only (triggers no banco).
2. **Integridade do cálculo de comissão** — apuração transacional e atômica.
3. **Precisão monetária sem `float`** — `int64` (centavos) + `NUMERIC`; verificado por AST, grep e Zod nas 3 camadas.
4. **RBAC deny-by-default** — autorização por papel (Gestor / Financeiro / Vendedor).
5. **Conformidade LGPD** — anonimização auditável.

---

## 🚀 Subindo o ambiente de desenvolvimento

### Pré-requisitos

- [Go](https://go.dev/) **1.26+**
- [Node.js](https://nodejs.org/) **20+** e npm
- [Docker](https://www.docker.com/) + Docker Compose
- [golang-migrate](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate) (CLI) — para aplicar as migrations
  ```bash
  # macOS
  brew install golang-migrate
  ```

> O PostgreSQL do Docker expõe a porta **5433** no host (para não colidir com um
> Postgres local em 5432). A connection string de dev usa, portanto, `:5433`.

### 1. Banco de dados (PostgreSQL via Docker)

```bash
docker compose up -d postgres
# Confirme: STATUS "healthy"
docker compose ps
```

Credenciais (definidas no `docker-compose.yml`):
`financialuser` / `financialpass` · banco `financial_dashboard` · porta `5433`.

### 2. Migrations + dados de seed

```bash
# Exporte a connection string de dev (porta 5433)
export DB_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable"

# Aplique as migrations (22 arquivos, FASES 1–10)
make -C backend migrate-up DB_URL="$DB_URL"

# Crie os usuários de demonstração (Gestor / Financeiro / Vendedor)
docker exec -i financial-dashboard-db \
  psql -U financialuser -d financial_dashboard < backend/testdata/seed_users.sql
```

### 3. Backend (API Go — porta 8080)

```bash
cd backend
DATABASE_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable" \
JWT_SECRET="dev-secret-troque-em-producao" \
JWT_ACCESS_TTL="1h" \
JWT_REFRESH_TTL="168h" \
RATE_LIMIT_MAX=100 \
ENVIRONMENT=development \
go run ./cmd/api
# → http://localhost:8080  (healthcheck: GET /ready)
```

### 4. Frontend (React + Vite — porta 5173)

```bash
cd web
npm install
npm run dev
# → http://localhost:5173  (proxy de /api → http://localhost:8080)
```

Abra **http://localhost:5173** e faça login.

### 🔑 Credenciais de demonstração

Todas com a senha **`E2ETest@2026!`**:

| Papel | E-mail | O que vê |
|-------|--------|----------|
| **Gestor** | `gestor-e2e@test.com` | Tudo: vendedores, pedidos, comissões, dashboard consolidado, apuração |
| **Financeiro** | `financeiro-e2e@test.com` | Controle de pagamentos/comissões + dashboards |
| **Vendedor** | `vendedor-user-e2e@test.com` | Apenas os próprios dados (dashboard, pedidos, comissões) |

> ⚠️ Senhas de demonstração — **troque antes de qualquer uso real**.

---

## 🧪 Testes

```bash
# Backend — unit
make -C backend test

# Backend — integração (precisa do Postgres no :5433)
make -C backend test-integration DB_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable"

# Frontend — unit (Vitest + jsdom)
cd web && npx vitest run

# E2E — API-driven (Playwright)
cd e2e && npm install && npx playwright install && npm test

# E2E — no navegador (assistir ao vivo). Requer backend :8080 + frontend :5173 no ar.
cd e2e && npm run test:ui          # Playwright UI mode (interativo)
cd e2e && npm run test:ui:headed   # abre o Chromium executando os specs
```

---

## 📂 Estrutura

```
.
├── backend/                 # API Go (chi + pgx)
│   ├── cmd/api/             #   entrypoint (main.go)
│   ├── internal/            #   domain, dto, repository, service, http, middleware
│   ├── migrations/          #   22 migrations SQL (golang-migrate)
│   ├── testdata/seed_users.sql
│   └── Makefile
├── web/                     # Frontend React + Vite + TS
│   └── src/                 #   api/ (hooks+zod), pages/, components/, types/
├── e2e/                     # Playwright (API-driven + browser-driven *.ui.spec.ts)
├── docs/                    # SDD: briefing, constitution, spec, plan, contracts, tasks
├── docker-compose.yml       # PostgreSQL (+ api/web/nginx no perfil de produção)
└── .claude/                 # trilha de auditoria da execução agente-00c
```

---

## ⚙️ Variáveis de ambiente (backend)

| Variável | Default | Descrição |
|----------|---------|-----------|
| `DATABASE_URL` | — | DSN do PostgreSQL (use `:5433` em dev) |
| `JWT_SECRET` | — | Segredo HS256 para assinar os tokens |
| `JWT_ACCESS_TTL` | `15m` | Validade do access token |
| `JWT_REFRESH_TTL` | `168h` | Validade do refresh token (cookie httpOnly) |
| `RATE_LIMIT_MAX` | `5` | Tentativas de login por IP / 60s |
| `ENVIRONMENT` | `development` | `production` ativa logs JSON estruturados |
| `TLS_CERT_PATH` / `TLS_KEY_PATH` | — | Se definidos, servem HTTPS |

---

## 📝 Notas

- **Commits locais**: o agente-00c nunca faz `git push` por design — todo o histórico
  foi gerado localmente.
- **Limitação conhecida (dev)**: em modo dev, o React StrictMode faz double-mount e
  o silent-refresh de sessão dispara uma chamada extra a `/auth/refresh` (um `401`
  benigno no console); não ocorre em produção.
- **Natureza**: PoC experimental gerada por IA. Revise segurança, validações e
  cobertura antes de qualquer uso além de demonstração.
