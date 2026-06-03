// Package internal — arquivo de dependências declaradas.
// Garante que todas as dependências do módulo permaneçam no go.mod
// mesmo antes de serem usadas nas fases de implementação.
//
// Este arquivo será removido quando as implementações das FASE 1-4
// importarem diretamente cada pacote.
package internal

import (
	_ "github.com/golang-jwt/jwt/v5"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/shopspring/decimal"
	_ "github.com/stretchr/testify/assert"
)
