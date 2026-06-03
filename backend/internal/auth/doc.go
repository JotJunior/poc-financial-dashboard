// Package auth implementa autenticação JWT HS256 com blocklist de tokens
// revogados (constitution P-IV — RBAC deny-by-default).
//
// Algoritmo fixado em HS256 (rejeita outros — OWASP JWT security).
// Hash de senha: Argon2id m=64MB t=3 p=4 (tarefa 0.5.2/2.1.1).
package auth
