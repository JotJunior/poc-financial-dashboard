// App.tsx — configuração do roteador com rotas protegidas por papel (Task 8.1.4)
// P-IV: UI é complementar — backend é a barreira real de RBAC
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Routes, Route, Navigate, Link } from 'react-router-dom';
import { AuthProvider, useAuth } from './api/auth-context';
import { ProtectedRoute } from './components/ProtectedRoute';
import { Layout } from './components/Layout';
import { Login } from './pages/Login';
import { Vendors } from './pages/Vendors';
import { VendorDetail } from './pages/VendorDetail';
import { Orders } from './pages/Orders';
import { Commissions } from './pages/Commissions';
import { DashboardConsolidated } from './pages/DashboardConsolidated';
import { DashboardVendor } from './pages/DashboardVendor';
import { PendingCommissions } from './pages/PendingCommissions';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
    },
  },
});

function NotFound() {
  return (
    <div style={{ textAlign: 'center', padding: '4rem', color: '#a6adc8' }}>
      <h1 style={{ color: '#cba6f7' }}>404</h1>
      <p>Página não encontrada.</p>
    </div>
  );
}

function Forbidden() {
  return (
    <div style={{ textAlign: 'center', padding: '4rem', color: '#a6adc8' }}>
      <h1 style={{ color: '#f38ba8' }}>403</h1>
      <p>Sem permissão para acessar esta página.</p>
      {/* Saída para não deixar o usuário preso (ex.: Vendedor que tocou uma
          rota de Gestor). Link '/' → DefaultDashboard → home do papel. */}
      <p style={{ marginTop: '1.5rem' }}>
        <Link to="/" style={{ color: '#cba6f7', fontWeight: 600 }}>
          ← Voltar ao meu painel
        </Link>
      </p>
    </div>
  );
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <Routes>
            {/* Pública */}
            <Route path="/login" element={<Login />} />
            <Route path="/403" element={<Forbidden />} />

            {/* Protegidas — exige autenticação */}
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <Layout />
                </ProtectedRoute>
              }
            >
              {/* Redirecionar / para dashboard apropriado */}
              <Route index element={<DefaultDashboard />} />

              {/* Dashboard Consolidado — Gestor e Financeiro */}
              <Route
                path="dashboard/consolidated"
                element={
                  <ProtectedRoute roles={['gestor', 'financeiro']}>
                    <DashboardConsolidated />
                  </ProtectedRoute>
                }
              />

              {/* Dashboard do Vendedor — todos os papéis (Vendedor: apenas os próprios) */}
              <Route path="dashboard/vendor" element={<DashboardVendor />} />

              {/* Comissões pendentes — Gestor e Financeiro */}
              <Route
                path="pending-commissions"
                element={
                  <ProtectedRoute roles={['gestor', 'financeiro']}>
                    <PendingCommissions />
                  </ProtectedRoute>
                }
              />

              {/* Vendedores — Gestor e Financeiro */}
              <Route
                path="vendors"
                element={
                  <ProtectedRoute roles={['gestor', 'financeiro']}>
                    <Vendors />
                  </ProtectedRoute>
                }
              />
              <Route
                path="vendors/:id"
                element={
                  <ProtectedRoute roles={['gestor', 'financeiro']}>
                    <VendorDetail />
                  </ProtectedRoute>
                }
              />

              {/* Pedidos — todos os papéis (Vendedor vê apenas os seus) */}
              <Route path="orders" element={<Orders />} />

              {/* Comissões — todos os papéis (Vendedor vê apenas as suas) */}
              <Route path="commissions" element={<Commissions />} />

              {/* 404 dentro do layout */}
              <Route path="*" element={<NotFound />} />
            </Route>

            {/* Fallback global */}
            <Route path="*" element={<Navigate to="/login" replace />} />
          </Routes>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  );
}

// Redirecionamento inteligente baseado no papel (Task 8.1.4)
// Vendedor → /dashboard/vendor; Gestor/Financeiro → /dashboard/consolidated
// user pode ser null se o token ainda não foi carregado — mas como o token é
// lido sincronamente do módulo client.ts no useState, user NUNCA é null
// quando isAuthenticated=true. Este componente só renderiza dentro do ProtectedRoute
// (que já garantiu isAuthenticated=true), então user.role é sempre definido.
function DefaultDashboard() {
  const { user } = useAuth();
  if (user?.role === 'vendedor') {
    return <Navigate to="/dashboard/vendor" replace />;
  }
  // Gestor, Financeiro, ou fallback
  return <Navigate to="/dashboard/consolidated" replace />;
}
