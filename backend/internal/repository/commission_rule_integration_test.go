//go:build integration

// Package repository — testes de integração para CommissionRuleRepository.
// Task 3.2.5: verificar criação de múltiplas versões, seleção correta por data,
// e imutabilidade das regras anteriores (trigger bloqueia UPDATE direto).
//
// Executar com:
//
//	DATABASE_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable" \
//	  go test -tags=integration ./internal/repository/... -run TestCommissionRule -v
package repository

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testVendorID é um UUID fixo para testes de commission_rule — isolado dos outros testes.
const testCommVendorID = "cccccccc-cccc-cccc-cccc-000000000001"

// setupCommissionRuleTest inicializa o pool e limpa dados de testes anteriores.
// Retorna o pool e uma função de teardown.
func setupCommissionRuleTest(t *testing.T) (*pgxpool.Pool, *PGCommissionRuleRepository) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL não definida — pulando teste de integração")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "falha ao conectar ao banco de dados")

	cleanup := func() {
		// session_replication_role='replica' desabilita triggers para DELETE de teste.
		// Necessário pois commission_rules tem trigger fn_prevent_mutation em DELETE.
		_, _ = pool.Exec(context.Background(), `
			DO $$ BEGIN
				SET LOCAL session_replication_role = 'replica';
				DELETE FROM commission_rules WHERE vendor_id = 'cccccccc-cccc-cccc-cccc-000000000001';
				DELETE FROM vendors WHERE id = 'cccccccc-cccc-cccc-cccc-000000000001';
				DELETE FROM users WHERE email = 'integ-commission-rule@test.com';
			END $$
		`)
	}

	// Limpar antes (idempotência entre runs).
	cleanup()
	t.Cleanup(func() {
		cleanup()
		pool.Close()
	})

	// Inserir usuário e vendedor de teste (FK necessária para commission_rules).
	actorID := "dddddddd-dddd-dddd-dddd-000000000001"
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, 'Gestor Comissão Teste', 'integ-commission-rule@test.com',
		        '$argon2id$v=19$m=65536,t=3,p=4$dGVzdA$dGVzdA', 'gestor')
		ON CONFLICT (email) DO NOTHING
	`, actorID)
	require.NoError(t, err, "inserir usuário ator de teste")

	_, err = pool.Exec(ctx, `
		INSERT INTO vendors (id, name, email, status)
		VALUES ($1, 'Vendedor Comissão Integ', 'vendor-commission-integ@test.com', 'ativo')
		ON CONFLICT (id) DO NOTHING
	`, testCommVendorID)
	require.NoError(t, err, "inserir vendedor de teste")

	repo := NewPGCommissionRuleRepository(pool)
	return pool, repo
}

// TestCommissionRule_CreateThreeVersions verifica que:
//   - Três versões são criadas com números incrementais (1, 2, 3)
//   - A regra anterior tem valid_to preenchido após criação de nova versão
//   - A versão mais recente tem valid_to = NULL (ainda ativa)
func TestCommissionRule_CreateThreeVersions(t *testing.T) {
	_, repo := setupCommissionRuleTest(t)
	ctx := context.Background()

	pct1 := decimal.NewFromFloat(8.0)
	pct2 := decimal.NewFromFloat(10.0)
	pct3 := decimal.NewFromFloat(12.5)

	// Criar 3 versões em sequência.
	rule1, err := repo.CreateRule(ctx, testCommVendorID, pct1)
	require.NoError(t, err, "criar regra v1")
	assert.Equal(t, 1, rule1.Version, "v1 deve ter version=1")
	assert.Nil(t, rule1.ValidTo, "v1 recém criada deve ter valid_to=NULL")
	assert.True(t, rule1.Percentage.Equal(pct1), "v1 percentual deve ser 8.0")

	rule2, err := repo.CreateRule(ctx, testCommVendorID, pct2)
	require.NoError(t, err, "criar regra v2")
	assert.Equal(t, 2, rule2.Version, "v2 deve ter version=2")
	assert.Nil(t, rule2.ValidTo, "v2 recém criada deve ter valid_to=NULL")
	assert.True(t, rule2.Percentage.Equal(pct2), "v2 percentual deve ser 10.0")

	rule3, err := repo.CreateRule(ctx, testCommVendorID, pct3)
	require.NoError(t, err, "criar regra v3")
	assert.Equal(t, 3, rule3.Version, "v3 deve ter version=3")
	assert.Nil(t, rule3.ValidTo, "v3 recém criada deve ter valid_to=NULL")
	assert.True(t, rule3.Percentage.Equal(pct3), "v3 percentual deve ser 12.5")

	// Verificar histórico completo via ListByVendor.
	all, err := repo.ListByVendor(ctx, testCommVendorID)
	require.NoError(t, err, "listar regras do vendedor")
	assert.Len(t, all, 3, "deve haver exatamente 3 regras")

	// Verificar que as regras anteriores têm valid_to preenchido.
	// ListByVendor retorna em ordem DESC por version.
	byVersion := make(map[int]CommissionRule, 3)
	for _, r := range all {
		byVersion[r.Version] = r
	}

	assert.NotNil(t, byVersion[1].ValidTo, "v1 deve ter valid_to preenchido após v2 ser criada")
	assert.NotNil(t, byVersion[2].ValidTo, "v2 deve ter valid_to preenchido após v3 ser criada")
	assert.Nil(t, byVersion[3].ValidTo, "v3 (mais recente) deve ter valid_to=NULL")
}

// TestCommissionRule_FindActiveAtDate_SelectsByDate verifica que FindActiveAtDate
// retorna a regra correta de acordo com a data informada (dec-020/A-005).
//
// Setup: inserir 3 regras com valid_from distintos via SQL direto (bypass trigger
// usando session_replication_role='replica' para controlar datas exatas).
// A criação via CreateRule sempre usa CURRENT_DATE — para testar seleção por data
// histórica, precisamos de datas no passado definidas explicitamente.
func TestCommissionRule_FindActiveAtDate_SelectsByDate(t *testing.T) {
	pool, repo := setupCommissionRuleTest(t)
	ctx := context.Background()

	// Inserir 3 regras com valid_from distintos diretamente no banco.
	// valid_from e valid_to são controlados para testar a seleção por data.
	//
	// Regra A: vigente de 2024-01-01 até 2024-06-30 (8%)
	// Regra B: vigente de 2024-07-01 até 2024-12-31 (10%)
	// Regra C: vigente de 2025-01-01 até NULL (12.5%) — ativa hoje
	_, err := pool.Exec(ctx, `
		INSERT INTO commission_rules (vendor_id, percentage, valid_from, valid_to, version)
		VALUES
		  ($1, 8.0000,  '2024-01-01', '2024-06-30', 10),
		  ($1, 10.0000, '2024-07-01', '2024-12-31', 11),
		  ($1, 12.5000, '2025-01-01', NULL,          12)
	`, testCommVendorID)
	require.NoError(t, err, "inserir regras históricas para teste de seleção por data")

	// Data no período da regra A.
	dateA := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	ruleA, err := repo.FindActiveAtDate(ctx, testCommVendorID, dateA)
	require.NoError(t, err, "FindActiveAtDate para data da regra A")
	assert.True(t, ruleA.Percentage.Equal(decimal.NewFromFloat(8.0)),
		"regra para 2024-03-15 deve ser 8.0%%, got=%s", ruleA.Percentage)
	assert.Equal(t, 10, ruleA.Version)

	// Data no período da regra B.
	dateB := time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)
	ruleB, err := repo.FindActiveAtDate(ctx, testCommVendorID, dateB)
	require.NoError(t, err, "FindActiveAtDate para data da regra B")
	assert.True(t, ruleB.Percentage.Equal(decimal.NewFromFloat(10.0)),
		"regra para 2024-09-01 deve ser 10.0%%, got=%s", ruleB.Percentage)
	assert.Equal(t, 11, ruleB.Version)

	// Data no período da regra C (ativa).
	dateC := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	ruleC, err := repo.FindActiveAtDate(ctx, testCommVendorID, dateC)
	require.NoError(t, err, "FindActiveAtDate para data da regra C")
	assert.True(t, ruleC.Percentage.Equal(decimal.NewFromFloat(12.5)),
		"regra para 2025-06-01 deve ser 12.5%%, got=%s", ruleC.Percentage)
	assert.Equal(t, 12, ruleC.Version)

	// Data fora de qualquer vigência — deve retornar ErrCommissionRuleNotFound.
	dateBefore := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
	_, err = repo.FindActiveAtDate(ctx, testCommVendorID, dateBefore)
	assert.ErrorIs(t, err, ErrCommissionRuleNotFound,
		"data anterior a qualquer regra deve retornar ErrCommissionRuleNotFound")
}

// TestCommissionRule_PreviousRuleIsImmutable verifica que uma tentativa de UPDATE
// direto em commission_rules falha com RAISE EXCEPTION do trigger fn_prevent_mutation.
// Constitution P-I NON-NEGOTIABLE: tabelas de trilha audit são imutáveis.
func TestCommissionRule_PreviousRuleIsImmutable(t *testing.T) {
	pool, repo := setupCommissionRuleTest(t)
	ctx := context.Background()

	// Criar uma regra primeiro.
	_, err := repo.CreateRule(ctx, testCommVendorID, decimal.NewFromFloat(5.0))
	require.NoError(t, err, "criar regra para teste de imutabilidade")

	// Tentar UPDATE direto (sem session_replication_role='replica').
	// O trigger fn_prevent_mutation deve bloquear esta operação.
	_, err = pool.Exec(ctx, `
		UPDATE commission_rules
		SET percentage = 99.0000
		WHERE vendor_id = $1
	`, testCommVendorID)

	// Deve falhar com erro do trigger.
	require.Error(t, err, "UPDATE direto deve ser bloqueado pelo trigger fn_prevent_mutation")
	assert.True(t,
		strings.Contains(err.Error(), "proibida") || strings.Contains(err.Error(), "imutavel") ||
			strings.Contains(err.Error(), "P-I") || strings.Contains(err.Error(), "mutation"),
		"erro deve conter mensagem do trigger fn_prevent_mutation, got: %s", err.Error())

	// Verificar que o valor original NÃO foi alterado (regra ainda íntegra).
	rules, err := repo.ListByVendor(ctx, testCommVendorID)
	require.NoError(t, err)
	for _, r := range rules {
		assert.False(t, r.Percentage.Equal(decimal.NewFromFloat(99.0)),
			"nenhuma regra deve ter percentage=99 após UPDATE bloqueado")
	}
}

// TestCommissionRule_ListByVendor_OrderedByVersionDesc verifica que ListByVendor
// retorna regras em ordem decrescente de versão (mais recente primeiro).
func TestCommissionRule_ListByVendor_OrderedByVersionDesc(t *testing.T) {
	_, repo := setupCommissionRuleTest(t)
	ctx := context.Background()

	// Criar 3 versões.
	_, err := repo.CreateRule(ctx, testCommVendorID, decimal.NewFromFloat(5.0))
	require.NoError(t, err)
	_, err = repo.CreateRule(ctx, testCommVendorID, decimal.NewFromFloat(7.5))
	require.NoError(t, err)
	_, err = repo.CreateRule(ctx, testCommVendorID, decimal.NewFromFloat(9.0))
	require.NoError(t, err)

	rules, err := repo.ListByVendor(ctx, testCommVendorID)
	require.NoError(t, err)
	require.Len(t, rules, 3)

	// version deve ser 3, 2, 1 (DESC).
	assert.Equal(t, 3, rules[0].Version, "primeiro deve ser versão mais recente (3)")
	assert.Equal(t, 2, rules[1].Version)
	assert.Equal(t, 1, rules[2].Version, "último deve ser versão mais antiga (1)")
}
