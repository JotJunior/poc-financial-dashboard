// ProtectedRoute — wrapper RBAC para rotas (Task 8.1.3)
// Verifica autenticação e papel antes de renderizar.
// P-IV: UI complementa segurança, backend é a barreira real.
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../api/auth-context';
import type { UserRole } from '../types/auth';

interface ProtectedRouteProps {
  children: React.ReactNode;
  /** Se definido, exige que o usuário tenha um desses papéis */
  roles?: UserRole[];
}

export function ProtectedRoute({ children, roles }: ProtectedRouteProps) {
  const { isAuthenticated, isLoading, user } = useAuth();
  const location = useLocation();

  if (isLoading) {
    // Sessão ainda sendo restaurada (silent refresh via cookie). Não redirecionar
    // para /login ainda — senão um reload sempre expulsaria o usuário logado.
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#1e1e2e',
          color: '#a6adc8',
        }}
      >
        Carregando…
      </div>
    );
  }

  if (!isAuthenticated) {
    // Redirecionar para login preservando a URL de destino
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  if (roles && user && !roles.includes(user.role)) {
    // Autenticado mas sem permissão — mostrar 403
    return <Navigate to="/403" replace />;
  }

  return <>{children}</>;
}
