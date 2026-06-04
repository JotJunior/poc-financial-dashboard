//go:build integration

// Package repository — schema integration tests.
// Task 1.4.11 / SC-007 / CHK063
// Verifica que nenhuma coluna monetária usa tipo float (real/double precision/float4/float8).
// Constitution P-III NON-NEGOTIABLE: zero float em colunas monetárias.
//
// Executar com: DATABASE_URL="..." go test ./... -v -tags integration -run TestSchema
package repository

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchema_NoFloatMonetaryColumns implementa SC-007 / CHK063.
// Varre information_schema.columns na schema 'public' e falha se
// qualquer coluna tiver data_type IN ('real','double precision','float4','float8').
func TestSchema_NoFloatMonetaryColumns(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL não definida — pulando teste de integração")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err, "falha ao conectar ao banco de dados")
	defer conn.Close(ctx)

	const query = `
		SELECT table_name, column_name, data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND data_type IN ('real', 'double precision', 'float4', 'float8')
		ORDER BY table_name, column_name
	`

	rows, err := conn.Query(ctx, query)
	require.NoError(t, err, "falha ao executar query SC-007")
	defer rows.Close()

	type floatColumn struct {
		TableName  string
		ColumnName string
		DataType   string
	}

	var violations []floatColumn
	for rows.Next() {
		var col floatColumn
		err := rows.Scan(&col.TableName, &col.ColumnName, &col.DataType)
		require.NoError(t, err)
		violations = append(violations, col)
	}
	require.NoError(t, rows.Err())

	assert.Empty(t, violations,
		"SC-007 VIOLAÇÃO: colunas com tipo float encontradas (constitution P-III NON-NEGOTIABLE). "+
			"Todas as colunas monetárias devem ser BIGINT (centavos) ou NUMERIC(7,4). "+
			"Violações: %+v", violations)
}

// TestSchema_MonetaryColumnsAreBigintOrNumeric verifica que colunas
// com sufixo '_cents' são BIGINT e colunas de percentual são NUMERIC.
func TestSchema_MonetaryColumnsAreBigintOrNumeric(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL não definida — pulando teste de integração")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err)
	defer conn.Close(ctx)

	// Verificar colunas _cents em TABELAS BASE: devem ser bigint
	// Views excluídas: SQL agregações (SUM) retornam NUMERIC por definição — ainda seguro.
	const centsCols = `
		SELECT c.table_name, c.column_name, c.data_type
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = 'public'
		  AND t.table_type = 'BASE TABLE'
		  AND c.column_name LIKE '%_cents'
		  AND c.data_type != 'bigint'
		ORDER BY c.table_name, c.column_name
	`

	rows, err := conn.Query(ctx, centsCols)
	require.NoError(t, err)
	defer rows.Close()

	type wrongCol struct {
		TableName  string
		ColumnName string
		DataType   string
	}

	var centsViolations []wrongCol
	for rows.Next() {
		var col wrongCol
		err := rows.Scan(&col.TableName, &col.ColumnName, &col.DataType)
		require.NoError(t, err)
		centsViolations = append(centsViolations, col)
	}
	require.NoError(t, rows.Err())

	assert.Empty(t, centsViolations,
		"SC-007: colunas com sufixo '_cents' devem ser BIGINT. "+
			"Violações: %+v", centsViolations)

	// Verificar colunas de percentual em TABELAS BASE: devem ser NUMERIC
	const pctCols = `
		SELECT c.table_name, c.column_name, c.data_type
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = 'public'
		  AND t.table_type = 'BASE TABLE'
		  AND c.column_name LIKE '%percentage%'
		  AND c.data_type NOT IN ('numeric', 'integer', 'bigint')
		ORDER BY c.table_name, c.column_name
	`

	rows2, err := conn.Query(ctx, pctCols)
	require.NoError(t, err)
	defer rows2.Close()

	var pctViolations []wrongCol
	for rows2.Next() {
		var col wrongCol
		err := rows2.Scan(&col.TableName, &col.ColumnName, &col.DataType)
		require.NoError(t, err)
		pctViolations = append(pctViolations, col)
	}
	require.NoError(t, rows2.Err())

	assert.Empty(t, pctViolations,
		"SC-007: colunas de percentual devem ser NUMERIC, não float. "+
			"Violações: %+v", pctViolations)
}
