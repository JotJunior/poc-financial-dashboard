<!--
Sync Impact Report
- Version: (none) → 1.0.0
- Tipo de bump: criacao inicial (MAJOR 1.0.0)
- Principios criados:
  - I. Auditabilidade Financeira Total (NON-NEGOTIABLE)
  - II. Integridade do Calculo de Comissao (NON-NEGOTIABLE)
  - III. Precisao Monetaria sem Ponto Flutuante (NON-NEGOTIABLE)
  - IV. Autorizacao por Papel (RBAC, Deny-by-Default)
  - V. Conformidade LGPD e Minimizacao de Dados
- Secoes adicionadas:
  - Core Principles (5 principios)
  - Padroes Tecnicos e de Qualidade
  - Restricoes de Escopo
  - Governance
- Secoes removidas: nenhuma
- Artefatos que precisam atualizacao (status):
  - CLAUDE.md — INEXISTENTE (criar quando houver codigo; refletir P-I a P-V)
  - docs/specs/*/spec.md — A CRIAR (etapa specify; herdar principios)
  - docs/specs/*/plan.md — A CRIAR (etapa plan; Constitution Check como gate)
  - docs/specs/*/tasks.md — A CRIAR (etapa create-tasks; quality gates derivados)
- TODOs pendentes:
  - TODO(AUTH_MECHANISM): mecanismo concreto de autenticacao a definir em clarify/plan
    (briefing classifica como "Itens a Definir / Medio"). Principio IV crava o
    contrato RBAC independente do mecanismo.
- Notas: gerado em modo autonomo (agente-00c, onda-002), sem operador presente.
  Suposicoes de negocio dec-003..dec-006 (atores, regra de comissao, estados)
  permanecem "a validar em clarify"; os principios abaixo cravam GOVERNANCA
  (invariantes que sobrevivem a qualquer resolucao das suposicoes), nao as
  regras de negocio especificas.
-->

# Financial Dashboard Constitution

Documento de principios imutaveis que governa decisoes de arquitetura, qualidade e
processo do Financial Dashboard — sistema de cadastro de vendedores, registro de
pedidos, calculo automatico de comissoes, dashboards de performance e controle de
pagamentos de comissoes (stack: Go + React + PostgreSQL).

A ordem de prioridade declarada no briefing governa qualquer trade-off entre
principios: **Corretude/Auditabilidade dos calculos financeiros > UX dos dashboards
> Velocidade de entrega > Amplitude de escopo.**

## Core Principles

### I. Auditabilidade Financeira Total (NON-NEGOTIABLE)

Toda grandeza financeira derivada MUST ser rastreavel ate suas origens primarias.

- Cada comissao apurada MUST referenciar, de forma persistente e imutavel, o pedido
  pago de origem, o percentual aplicado e a versao da regra de comissao vigente no
  momento da apuracao (Why: reapuracao e contestacao exigem reconstruir o calculo
  exato, mesmo apos a regra mudar).
- Cada transicao de estado de um pagamento de comissao (`pendente` → `aprovado` →
  `pago`, incluindo reversoes) MUST ser registrada em trilha append-only com ator,
  timestamp (UTC) e motivo. Registros de trilha MUST NOT ser editados ou removidos.
- Nenhum valor financeiro consolidado exibido em dashboard MUST existir sem que seja
  derivavel deterministicamente dos registros transacionais subjacentes.

Teste de conformidade: dado qualquer valor de comissao ou pagamento, e possivel
produzir a cadeia completa de evidencias (pedido → percentual → regra → transicoes)
sem inferencia.

### II. Integridade do Calculo de Comissao (NON-NEGOTIABLE)

O motor de comissoes MUST ser deterministico, idempotente e reproduzivel.

- Dado o mesmo conjunto de pedidos pagos e a mesma versao de regra, o calculo MUST
  produzir exatamente o mesmo resultado em qualquer execucao (determinismo).
- Reexecutar a apuracao de um periodo ja apurado MUST NOT duplicar nem alterar
  comissoes ja persistidas; deve ser uma operacao idempotente (Why: retry/reprocesso
  sem corrupcao de estado financeiro).
- Toda regra de comissao MUST ser versionada; uma comissao MUST registrar qual versao
  de regra a originou. Mudanca de regra MUST NOT alterar retroativamente comissoes ja
  apuradas, salvo reapuracao explicita e auditada.
- Comissao so MUST ser devida sobre pedido em estado `pago`; pedidos em outros estados
  MUST NOT gerar comissao.

Teste de conformidade: existe teste automatizado que apura o mesmo periodo duas vezes
e verifica resultado identico e ausencia de duplicacao.

### III. Precisao Monetaria sem Ponto Flutuante (NON-NEGOTIABLE)

Valores monetarios MUST NOT ser representados, armazenados ou calculados como ponto
flutuante binario (`float`/`double`/`real`).

- Persistencia MUST usar `NUMERIC`/`DECIMAL` (PostgreSQL) com escala explicita, OU
  inteiro de centavos. Colunas monetarias MUST NOT usar `float`/`double precision`.
- No backend (Go), valores monetarios MUST trafegar em tipo decimal/inteiro de
  centavos — nunca `float64` — desde a borda da API ate a persistencia.
- A politica de arredondamento MUST ser unica, explicita e documentada; arredondamento
  MUST ocorrer em pontos definidos e testados (Why: erros de arredondamento corroem a
  confianca financeira e violam o Principio I).

Teste de conformidade: schema sem colunas float em campos monetarios; testes de
calculo cobrem casos de arredondamento de fronteira.

### IV. Autorizacao por Papel (RBAC, Deny-by-Default)

Acesso a dados e operacoes MUST ser controlado por papel, negando por padrao.

- Papeis e fronteiras MUST ser respeitados em TODA operacao:
  - **Gestor/Admin**: administracao geral (vendedores, pedidos, configuracao de
    percentuais, todos os dashboards e comissoes).
  - **Vendedor**: acesso SOMENTE as proprias metricas e proprias comissoes; MUST NOT
    visualizar dados de outros vendedores.
  - **Financeiro**: controle de pagamentos de comissoes (aprovar/marcar como pago);
    MUST NOT alterar regras de comissao nem cadastros de vendas.
- Autorizacao MUST ser deny-by-default: ausencia de permissao explicita nega o acesso.
- A autorizacao MUST ser aplicada no backend (server-side); controles apenas de UI
  MUST NOT ser considerados barreira de seguranca.
- O contrato de papeis acima e governanca cravada; o conjunto exato de atores pode ser
  confirmado/expandido em clarify, mas qualquer ator novo MUST receber escopo explicito
  least-privilege.

Teste de conformidade: requisicao de Vendedor a dado de outro vendedor retorna negacao;
endpoints de pagamento rejeitam papel nao-Financeiro.

### V. Conformidade LGPD e Minimizacao de Dados

Dados pessoais de vendedores (nome, contato, identificadores) MUST ser tratados sob a
LGPD com minimizacao.

- O sistema MUST coletar e persistir apenas os dados pessoais necessarios as funcoes do
  MVP (cadastro, contato, identificacao para apuracao). Coleta especulativa MUST NOT
  ocorrer.
- Acesso a dados pessoais MUST seguir o Principio IV (least-privilege por papel).
- Operacoes sobre dados pessoais MUST ser auditaveis (alinhado ao Principio I) para
  suportar direitos do titular e prestacao de contas.
- PCI-DSS NAO se aplica ao MVP (nao ha processamento de cartao); caso entre em escopo
  futuro, MUST ser tratado por amendment a esta constitution.

Teste de conformidade: inventario de dados pessoais mapeia cada campo a uma finalidade
explicita do MVP.

## Padroes Tecnicos e de Qualidade

- **Stack fixada**: Backend Go, Frontend React, Banco PostgreSQL. Desvios MUST ser
  justificados via amendment.
- **Testes do motor financeiro**: a logica de calculo de comissao e as maquinas de
  estado de pedido/pagamento MUST ter cobertura automatizada, incluindo casos de
  arredondamento e idempotencia (suporta Principios II e III).
- **Maquinas de estado explicitas**: transicoes de estado de Pedido e de Pagamento de
  comissao MUST ser implementadas como transicoes validadas; transicoes invalidas MUST
  ser rejeitadas, nao silenciosamente ignoradas.
- **Observabilidade de operacoes criticas**: apuracao de comissao e mudancas de estado
  de pagamento MUST emitir logs estruturados suficientes para reconstruir a trilha de
  auditoria (suporta Principio I).
- **Integridade relacional**: relacoes pedido↔vendedor↔comissao↔pagamento MUST ser
  protegidas por constraints no banco (FKs, checks de estado), nao apenas em codigo.

## Restricoes de Escopo

Fora de escopo do MVP (alteracoes requerem amendment ou nova spec):

- Integracao com ERP / sistemas de contabilidade externa.
- Emissao fiscal / NF-e.
- Multi-moeda.
- Multi-tenant.

Itens desejaveis pos-MVP (metas de venda, comissao por faixa/produto, exportacao de
relatorios, notificacoes) MUST respeitar todos os Core Principles quando implementados.

## Governance

- Esta constitution prevalece sobre qualquer outra pratica ou documento do projeto em
  caso de conflito. Specs, plans e tasks MUST estar alinhados a ela.
- **Constitution Check**: planos tecnicos (`plan.md`) MUST conter verificacao explicita
  de alinhamento aos Core Principles antes da implementacao. Violacao de principio
  NON-NEGOTIABLE (I, II, III) bloqueia a aprovacao do plano.
- **Amendments**: alteracoes MUST ser versionadas via SemVer e acompanhadas de Sync
  Impact Report listando artefatos impactados.
  - MAJOR: remocao ou redefinicao incompativel de principio.
  - MINOR: novo principio ou expansao material de secao.
  - PATCH: clarificacao/correcao sem mudanca semantica.
- **Excecoes**: desvio de um principio SHOULD/NON-NEGOTIABLE MUST ser registrado como
  decisao auditavel com justificativa e, para NON-NEGOTIABLE, aprovacao explicita do
  Gestor/Admin (stakeholder de decisao).
- **Suposicoes pendentes**: as regras de negocio inferidas (atores, regra de comissao,
  periodicidade de apuracao, estados de pedido/pagamento — dec-003..dec-006) permanecem
  a validar em `/clarify`. Sua resolucao MUST NOT violar os Core Principles; em caso de
  tensao, prevalece a ordem de prioridade (corretude/auditabilidade primeiro).

**Version**: 1.0.0 | **Ratified**: 2026-06-03 | **Last Amended**: 2026-06-03
