// Layout — navbar + menu por papel (Task 8.1.2)
// Gestor: acesso total; Vendedor: apenas próprio dashboard; Financeiro: comissões
import React from 'react';
import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from '../api/auth-context';
import { useLogout } from '../api/auth';
import type { UserRole } from '../types/auth';

const ROLE_LABELS: Record<UserRole, string> = {
  gestor: 'Gestor',
  vendedor: 'Vendedor',
  financeiro: 'Financeiro',
};

const ROLE_COLORS: Record<UserRole, string> = {
  gestor: '#7c3aed',
  vendedor: '#0891b2',
  financeiro: '#059669',
};

export function Layout() {
  const { user, clearAuth } = useAuth();
  const logout = useLogout();
  const navigate = useNavigate();

  const role = user?.role ?? 'vendedor';

  async function handleLogout() {
    await logout.mutateAsync();
    // Limpar o estado React do AuthContext (setAccessToken(null) no mutationFn
    // só limpa o módulo; clearAuth() atualiza o state React, garantindo que
    // isAuthenticated=false antes do navigate — previne redirect-loop no Login.)
    clearAuth();
    navigate('/login', { replace: true });
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <nav style={{
        background: '#1e1e2e',
        color: '#cdd6f4',
        padding: '0 1.5rem',
        display: 'flex',
        alignItems: 'center',
        gap: '1.5rem',
        height: '56px',
        boxShadow: '0 1px 3px rgba(0,0,0,0.4)',
      }}>
        {/* Logo */}
        <span style={{ fontWeight: 700, fontSize: '1.1rem', color: '#cba6f7', marginRight: '0.5rem' }}>
          FinDash
        </span>

        {/* Links por papel */}
        <NavLinks role={role} />

        {/* Spacer */}
        <div style={{ flex: 1 }} />

        {/* Badge de papel */}
        <span style={{
          background: ROLE_COLORS[role],
          color: '#fff',
          borderRadius: '9999px',
          padding: '2px 10px',
          fontSize: '0.75rem',
          fontWeight: 600,
        }}>
          {ROLE_LABELS[role]}
        </span>

        {/* Usuário */}
        <span style={{ fontSize: '0.85rem', color: '#a6adc8' }}>
          {user?.sub?.slice(0, 8)}…
        </span>

        {/* Logout */}
        <button
          onClick={handleLogout}
          disabled={logout.isPending}
          style={{
            background: 'transparent',
            border: '1px solid #585b70',
            color: '#cdd6f4',
            borderRadius: '6px',
            padding: '4px 10px',
            cursor: 'pointer',
            fontSize: '0.8rem',
          }}
        >
          {logout.isPending ? 'Saindo…' : 'Sair'}
        </button>
      </nav>

      <main style={{ flex: 1, padding: '1.5rem', background: '#1e1e2e', color: '#cdd6f4' }}>
        <Outlet />
      </main>
    </div>
  );
}

function NavLinks({ role }: { role: UserRole }) {
  const linkStyle: React.CSSProperties = {
    color: '#a6adc8',
    textDecoration: 'none',
    fontSize: '0.9rem',
    padding: '4px 8px',
    borderRadius: '4px',
    transition: 'color 0.15s',
  };

  const activeStyle: React.CSSProperties = { ...linkStyle, color: '#cba6f7' };
  void activeStyle; // usado via className em prod — aqui simplificado

  if (role === 'vendedor') {
    return (
      <>
        <Link to="/dashboard/vendor" style={linkStyle}>Meu Dashboard</Link>
        <Link to="/orders" style={linkStyle}>Pedidos</Link>
        <Link to="/commissions" style={linkStyle}>Minhas Comissões</Link>
      </>
    );
  }

  if (role === 'financeiro') {
    return (
      <>
        <Link to="/dashboard/consolidated" style={linkStyle}>Dashboard</Link>
        <Link to="/commissions" style={linkStyle}>Comissões</Link>
        <Link to="/pending-commissions" style={linkStyle}>Pendentes</Link>
      </>
    );
  }

  // Gestor — acesso total
  return (
    <>
      <Link to="/dashboard/consolidated" style={linkStyle}>Dashboard</Link>
      <Link to="/vendors" style={linkStyle}>Vendedores</Link>
      <Link to="/orders" style={linkStyle}>Pedidos</Link>
      <Link to="/commissions" style={linkStyle}>Comissões</Link>
      <Link to="/pending-commissions" style={linkStyle}>Pendentes</Link>
    </>
  );
}
