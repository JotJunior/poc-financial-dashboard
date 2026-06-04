//go:build integration

// Package repository — testes de integração para OrderRepository.
// Task 4.1.6: criar pedido; transicionar rascunho→confirmado→pago; tentar pago→rascunho → erro;
// verificar audit_trail com registros de transição.
//
// Executar com:
//
//	DATABASE_URL="postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable" \
//	  go test -tags=integration ./internal/repository/... -run TestOrder -v
package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"financial-dashboard/backend/internal/domain"
)

// IDs fixos para testes de order — isolados de outros testes.
const (
	orderTestVendorID = "eeeeeeee-eeee-eeee-eeee-000000000001"
	orderTestActorID  = "ffffffff-ffff-ffff-ffff-000000000001"
)

// setupOrderInteg inicializa o pool, insere vendedor/usuário de teste, e limpa ao final.
func setupOrderInteg(t *testing.T) (*pgxpool.Pool, *PGOrderRepository) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL não definida — pulando teste de integração")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "conectar ao banco de dados")

	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `
			DO $$ BEGIN
				SET LOCAL session_replication_role = 'replica';
				DELETE FROM audit_trail WHERE entity_id IN (
					SELECT id FROM orders WHERE vendor_id = 'eeeeeeee-eeee-eeee-eeee-000000000001'
				);
				DELETE FROM order_items WHERE order_id IN (
					SELECT id FROM orders WHERE vendor_id = 'eeeeeeee-eeee-eeee-eeee-000000000001'
				);
				DELETE FROM orders WHERE vendor_id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
				DELETE FROM vendors WHERE id = 'eeeeeeee-eeee-eeee-eeee-000000000001';
				DELETE FROM users WHERE id = 'ffffffff-ffff-ffff-ffff-000000000001';
			END $$
		`)
	}

	cleanup()
	t.Cleanup(func() {
		cleanup()
		pool.Close()
	})

	// Inserir usuário ator e vendedor de teste.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, email, password_hash, role)
		VALUES ($1, 'Gestor Order Test', 'integ-order-actor@test.com',
		        '$argon2id$v=19$m=65536,t=3,p=4$dGVzdA$dGVzdA', 'gestor')
		ON CONFLICT (id) DO NOTHING
	`, orderTestActorID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO vendors (id, name, email, status)
		VALUES ($1, 'Vendedor Order Integ', 'vendor-order-integ@test.com', 'ativo')
		ON CONFLICT (id) DO NOTHING
	`, orderTestVendorID)
	require.NoError(t, err)

	return pool, NewPGOrderRepository(pool)
}

// TestOrder_Create verifica criação de pedido com items.
func TestOrder_Create(t *testing.T) {
	_, repo := setupOrderInteg(t)
	ctx := context.Background()

	req := CreateOrderReq{
		VendorID:   orderTestVendorID,
		TotalCents: 50000, // R$ 500,00
		OrderDate:  time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Items: []CreateOrderItemReq{
			{Description: "Produto A", Quantity: 2, UnitPriceCents: 20000, LineTotalCents: 40000},
			{Description: "Produto B", Quantity: 1, UnitPriceCents: 10000, LineTotalCents: 10000},
		},
	}

	order, err := repo.Create(ctx, req)
	require.NoError(t, err)

	assert.NotEmpty(t, order.ID)
	assert.Equal(t, domain.OrderStatusRascunho, order.Status, "status inicial deve ser rascunho")
	assert.Equal(t, int64(50000), order.TotalCents)
	assert.Equal(t, orderTestVendorID, order.VendorID)
	assert.Len(t, order.Items, 2, "deve ter 2 itens")
	assert.Nil(t, order.PaidAt, "paid_at deve ser nil no rascunho")
}

// TestOrder_Transition_Rascunho_to_Confirmado_to_Pago verifica transição completa
// e cria audit_trail para cada transição (P-I).
func TestOrder_Transition_Rascunho_to_Confirmado_to_Pago(t *testing.T) {
	pool, repo := setupOrderInteg(t)
	ctx := context.Background()

	// Criar pedido.
	order, err := repo.Create(ctx, CreateOrderReq{
		VendorID:   orderTestVendorID,
		TotalCents: 100000,
		OrderDate:  time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	// rascunho → confirmado.
	confirmed, err := repo.Transition(ctx, order.ID, domain.OrderStatusConfirmado, orderTestActorID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusConfirmado, confirmed.Status)

	// confirmado → pago.
	paid, err := repo.Transition(ctx, order.ID, domain.OrderStatusPago, orderTestActorID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusPago, paid.Status)
	assert.NotNil(t, paid.PaidAt, "paid_at deve ser preenchido ao pagar")

	// Verificar audit_trail com 2 registros (P-I — trilha imutável).
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_trail
		WHERE entity_type = 'order' AND entity_id = $1
	`, order.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "deve haver 2 registros em audit_trail (rascunho→confirmado e confirmado→pago)")
}

// TestOrder_Transition_InvalidTransition verifica que transições inválidas são rejeitadas.
func TestOrder_Transition_InvalidTransition(t *testing.T) {
	_, repo := setupOrderInteg(t)
	ctx := context.Background()

	// Criar pedido e transicionar para pago.
	order, err := repo.Create(ctx, CreateOrderReq{
		VendorID:   orderTestVendorID,
		TotalCents: 20000,
		OrderDate:  time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	_, err = repo.Transition(ctx, order.ID, domain.OrderStatusConfirmado, orderTestActorID)
	require.NoError(t, err)
	_, err = repo.Transition(ctx, order.ID, domain.OrderStatusPago, orderTestActorID)
	require.NoError(t, err)

	// Tentar pago → rascunho (inválida).
	_, err = repo.Transition(ctx, order.ID, domain.OrderStatusRascunho, orderTestActorID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidTransition),
		"transição inválida deve retornar ErrInvalidTransition, got: %v", err)
}

// TestOrder_FindByID verifica busca com items de linha.
func TestOrder_FindByID(t *testing.T) {
	_, repo := setupOrderInteg(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, CreateOrderReq{
		VendorID:   orderTestVendorID,
		TotalCents: 30000,
		OrderDate:  time.Now().UTC(),
		Items: []CreateOrderItemReq{
			{Description: "Item Único", Quantity: 3, UnitPriceCents: 10000, LineTotalCents: 30000},
		},
	})
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Len(t, found.Items, 1)
	assert.Equal(t, "Item Único", found.Items[0].Description)
}

// TestOrder_FindByID_NotFound verifica retorno de ErrOrderNotFound.
func TestOrder_FindByID_NotFound(t *testing.T) {
	_, repo := setupOrderInteg(t)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, ErrOrderNotFound)
}
