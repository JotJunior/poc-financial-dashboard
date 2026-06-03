// Tipos de Autenticação — espelha backend/internal/dto/auth.go
// Ref: contracts/api.md §/auth; spec §FR-024-026; constitution P-IV

export type UserRole = 'gestor' | 'vendedor' | 'financeiro';

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  accessToken: string; // JWT HS256 — armazenar em memória, nunca localStorage
  // refresh_token retornado em httpOnly cookie (CHK026)
}

export interface AuthUser {
  sub: string; // UUID do usuário
  role: UserRole;
  vendorId: string | null; // não-nulo apenas para role='vendedor'
  exp: number; // Unix timestamp
}
