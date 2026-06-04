// AuthContext — gestão de sessão em memória (CHK026)
// Fornece AuthUser decodificado do JWT access_token.
// Token NUNCA em localStorage/sessionStorage — apenas em memória.
import React, { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { getAccessToken, setAccessToken } from './client';
import { refreshAccessToken } from './auth';
import type { AuthUser, UserRole } from '../types/auth';

// ─── Rota inicial por papel (role-aware) ──────────────────────────────────────
// Vendedor → seu próprio dashboard; Gestor/Financeiro → consolidado.
// Centralizado aqui para Login e DefaultDashboard usarem a MESMA regra,
// evitando a corrida que mandava o Vendedor por /dashboard/consolidated → /403.
export function homePathForRole(role: UserRole | undefined): string {
  return role === 'vendedor' ? '/dashboard/vendor' : '/dashboard/consolidated';
}

// ─── Decodificar JWT sem biblioteca (payload Base64URL → JSON) ────────────────

export function decodeJwtPayload(token: string): AuthUser | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = parts[1];
    // Padding Base64URL → Base64
    const padded = payload.replace(/-/g, '+').replace(/_/g, '/');
    const decoded = atob(padded);
    return JSON.parse(decoded) as AuthUser;
  } catch {
    return null;
  }
}

// ─── Context ──────────────────────────────────────────────────────────────────

export interface AuthContextValue {
  user: AuthUser | null;
  isAuthenticated: boolean;
  /** true enquanto a sessão é restaurada no mount (silent refresh via cookie) */
  isLoading: boolean;
  setToken: (token: string | null) => void;
  clearAuth: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(() => {
    // Ao montar: verificar se já tem token em memória (caso de hot-reload dev)
    const token = getAccessToken();
    return token ? decodeJwtPayload(token) : null;
  });
  // Carregando enquanto não há token em memória e ainda vamos tentar restaurar
  // a sessão pelo refresh-cookie httpOnly. Sem isso, um hard reload caía direto
  // no /login (token só vive em memória — CHK026).
  const [isLoading, setIsLoading] = useState<boolean>(() => !getAccessToken());

  const setToken = useCallback((token: string | null) => {
    setAccessToken(token);
    setUser(token ? decodeJwtPayload(token) : null);
  }, []);

  const clearAuth = useCallback(() => {
    setAccessToken(null);
    setUser(null);
  }, []);

  // Restauração de sessão no mount.
  useEffect(() => {
    let cancelled = false;
    const token = getAccessToken();

    if (token) {
      // Já há token em memória (hot-reload dev): apenas validar expiração.
      const decoded = decodeJwtPayload(token);
      if (!decoded || decoded.exp * 1000 < Date.now()) {
        clearAuth();
      }
      setIsLoading(false);
      return;
    }

    // Sem token em memória (hard reload / nova aba): tentar silent refresh
    // usando o refresh_token no cookie httpOnly. O browser envia o cookie
    // automaticamente; nunca o tocamos em JS (CHK026).
    (async () => {
      const newToken = await refreshAccessToken();
      if (cancelled) return;
      if (newToken) setUser(decodeJwtPayload(newToken));
      setIsLoading(false);
    })();

    return () => {
      cancelled = true;
    };
  }, [clearAuth]);

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, isLoading, setToken, clearAuth }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider');
  return ctx;
}
