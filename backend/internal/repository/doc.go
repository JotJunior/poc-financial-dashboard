// Package repository implementa acesso ao banco de dados PostgreSQL
// usando pgx/v5 com SQL explícito (sem ORM).
//
// Convenção: todas as queries usam named params via pgx.NamedArgs
// para evitar SQL injection. Snake_case (colunas DB) ↔ camelCase (DTOs Go).
package repository
