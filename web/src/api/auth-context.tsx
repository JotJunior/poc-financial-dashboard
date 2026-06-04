// AuthContext — gestão de sessão em memória (CHK026)
// Fornece AuthUser decodificado do JWT access_token.
// Token NUNCA em localStorage/sessionStorage — apenas em memória.
import React, { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { getAccessToken, setAccessToken } from './client';
import type { AuthUser } from '../types/auth';

// ─── Decodificar JWT sem biblioteca (payload Base64URL → JSON) ────────────────

function decodeJwtPayload(token: string): AuthUser | null {
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

  const setToken = useCallback((token: string | null) => {
    setAccessToken(token);
    setUser(token ? decodeJwtPayload(token) : null);
  }, []);

  const clearAuth = useCallback(() => {
    setAccessToken(null);
    setUser(null);
  }, []);

  // Limpar auth se token expirado ao montar
  useEffect(() => {
    const token = getAccessToken();
    if (token) {
      const decoded = decodeJwtPayload(token);
      if (!decoded || decoded.exp * 1000 < Date.now()) {
        clearAuth();
      }
    }
  }, [clearAuth]);

  return (
    <AuthContext.Provider value={{ user, isAuthenticated: !!user, setToken, clearAuth }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider');
  return ctx;
}
