# Project Briefing: Financial Dashboard

**Data**: 2026-06-03
**Status**: Draft
**Versao**: 1.0
**Modo de geracao**: Autonomo (agente-00c, onda-001) — sem operador presente. Atores e regras de negocio marcados `[suposicao a validar]` foram inferidos de padroes de mercado para sistemas de vendas/comissoes e registrados como Decisoes auditaveis (dec-003 a dec-006).

---

## 1. Visao e Proposito

**O que e**: Aplicacao para cadastro de vendedores e registro de pedidos, que entrega dashboards com metricas de performance de vendas, calcula comissoes automaticamente e controla o pagamento dessas comissoes.

**Problema que resolve**: Equipes de vendas com comissionamento carecem de uma fonte unica e confiavel para acompanhar performance, apurar comissoes corretamente e rastrear pagamentos — hoje frequentemente feito em planilhas manuais sujeitas a erro e sem trilha de auditoria.

**Proposta de valor**: Calculo de comissao automatico e auditavel sobre pedidos pagos, dashboards de performance por vendedor e consolidados, e um fluxo de controle de pagamentos de comissoes com estados claros.

## 2. Usuarios e Stakeholders

| Ator | Papel | Acoes Principais |
|------|-------|-----------------|
| Gestor/Admin `[suposicao a validar]` | Administracao geral | Cadastra vendedores, registra pedidos, configura percentuais de comissao, acompanha TODOS os dashboards e comissoes |
| Vendedor `[suposicao a validar]` | Usuario final (vendas) | Visualiza apenas suas proprias metricas de performance e suas comissoes |
| Financeiro `[suposicao a validar]` | Controle financeiro | Controla e registra pagamentos das comissoes (aprova / marca como pago) |

**Stakeholders de decisao**: Gestor/Admin define prioridades e aprova as regras de comissao.

## 3. Escopo

### MVP (Essencial)

1. Cadastro de vendedores (CRUD, com percentual de comissao por vendedor).
2. Registro de pedidos (vendedor, valor, itens, data, status).
3. Calculo automatico de comissao = percentual configuravel aplicado sobre o valor do pedido pago.
4. Dashboards de performance de vendas (por vendedor e consolidado, por periodo).
5. Controle de pagamentos de comissoes (apuracao mensal; status pendente / aprovado / pago).

### Pos-MVP (Desejavel)

1. Metas de venda por vendedor/periodo.
2. Comissao por faixa de valor e/ou por produto.
3. Exportacao de relatorios.
4. Notificacoes (ex: comissao aprovada, pagamento efetuado).

### Fora de Escopo (MVP)

- Integracao com ERP / sistemas de contabilidade externa.
- Emissao fiscal / NF-e.
- Multi-moeda.
- Multi-tenant.

## 4. Prioridades e Trade-offs

**Ordem de prioridade**: Corretude/Auditabilidade dos calculos financeiros > UX dos dashboards > Velocidade de entrega > Amplitude de escopo.

**Decisoes explicitas**:
- Aceitar escopo MVP enxuto (5 features) para garantir corretude do motor de comissoes.
- Precisao monetaria sem ponto flutuante (usar decimal ou inteiro de centavos) — trade-off de simplicidade por integridade financeira.

## 5. Restricoes

| Restricao | Valor | Notas |
|-----------|-------|-------|
| Prazo | Flexivel | POC / laboratorio |
| Equipe | Enxuta | Contexto experimental |
| Budget | Nao definido | — |
| Tecnica | Stack fixada: Go + React + PostgreSQL | Sugerida no init da execucao |

## 6. Stack Tecnica

| Camada | Tecnologia | Justificativa |
|--------|-----------|---------------|
| Backend | Go | API REST + motor de calculo de comissoes; performance e tipagem forte para logica financeira |
| Frontend | React | Dashboards interativos de metricas de performance |
| Banco de dados | PostgreSQL | Dados transacionais de vendas/pedidos/pagamentos com integridade relacional e suporte a tipos decimais precisos |
| Infraestrutura | `[inferido]` Containers (Docker) para dev/POC | Padrao para stack Go+React+Postgres; a definir |
| Integracoes | Nenhuma no MVP | ERP/fiscal explicitamente fora de escopo |

## 7. Qualidade e Padroes

**Padroes adotados**:
- Auditabilidade dos calculos de comissao — cada comissao apurada deve ser rastreavel ate o pedido pago de origem e ao percentual aplicado.
- Trilha de pagamentos — historico de transicoes de estado de cada pagamento de comissao.
- Precisao monetaria — valores monetarios NUNCA em float; usar decimal/numeric (Postgres `NUMERIC`) ou inteiro de centavos.

**Compliance**: LGPD aplicavel a dados pessoais de vendedores (nome, contato, identificadores). Sem PCI-DSS no MVP (nao ha processamento de cartao).

## 8. Visao de Futuro

**6 meses**: MVP operacional — cadastro de vendedores, registro de pedidos, calculo automatico de comissao, dashboards de performance e apuracao mensal de pagamentos.

**12 meses**: Metas de venda, comissao por faixa/produto, relatorios exportaveis e notificacoes.

**Riscos conhecidos**:
- Regras de comissao podem evoluir (faixa/produto) — mitigado adotando percentual configuravel desde o MVP.
- Precisao monetaria — risco de erros de arredondamento; mitigado por decimal/centavos e testes de calculo.
- Escopo de relatorios pode crescer alem do MVP — manter relatorios pos-MVP.

---

## Regras de Negocio Assumidas (suposicoes a validar)

> Inferidas de padroes de mercado e registradas como Decisoes auditaveis no state.json (dec-003 a dec-006). Devem ser confirmadas em `/clarify` antes de virarem requisitos cravados.

- **Comissao** = percentual configuravel (por vendedor; fallback global) aplicado sobre o valor total do pedido **pago**. (dec-004)
- **Apuracao** de comissoes **mensal**. (dec-005)
- **Estados de Pedido**: `rascunho`, `confirmado`, `pago`, `cancelado`. Comissao so e devida sobre pedido `pago`. (dec-005)
- **Estados de Pagamento de comissao**: `pendente`, `aprovado`, `pago`. (dec-005)
- **Atores e permissoes**: Gestor/Admin (tudo), Vendedor (somente proprias metricas/comissoes), Financeiro (pagamentos). (dec-003)

---

## Itens a Definir

| Item | Dimensao | Impacto |
|------|----------|---------|
| Confirmar atores e matriz de permissoes (3 atores inferidos) | Usuarios | Alto |
| Confirmar regra de comissao (%-sobre-pedido-pago vs faixa/produto) | Escopo/Negocio | Alto |
| Confirmar periodicidade de apuracao (mensal assumida) | Negocio | Medio |
| Confirmar maquinas de estado de Pedido e Pagamento | Negocio | Alto |
| Definir infraestrutura de deploy (Docker/cloud) | Tecnico | Baixo |
| Definir mecanismo de autenticacao/autorizacao | Tecnico/Seguranca | Medio |

---

## Setup / Bootstrap

Projeto **single-workspace** por camada (um modulo Go no backend, um app React no frontend) — NAO e monorepo multi-workspace npm/go.work/cargo. Portanto **nao** ha `scripts/bootstrap-deps.sh`: a instalacao de dependencias (`go mod download`, `npm install` do frontend) e tratada onda-a-onda na pipeline, sem amplificacao de bloqueios cirurgicos.

---

**Proximo passo recomendado**: `/constitution` para definir principios de governanca (auditabilidade financeira, precisao monetaria, LGPD).
