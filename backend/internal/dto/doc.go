// Package dto contém os Data Transfer Objects com json tags camelCase
// para serialização na API REST.
//
// Convenção: valores monetários são int64 (centavos), percentuais são
// string ("5.5000") para preservar precisão na serialização JSON.
package dto
