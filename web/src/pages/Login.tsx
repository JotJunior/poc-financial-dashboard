// Login — formulário com validação Zod + gestão de token em memória (Task 8.1.1)
// CHK026: access_token armazenado APENAS em memória via setToken(AuthContext)
import { useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useForm } from '../hooks/useForm';
import { useLogin } from '../api/auth';
import { useAuth, decodeJwtPayload, homePathForRole } from '../api/auth-context';
import { z } from 'zod';

const loginSchema = z.object({
  email: z.string().email('E-mail inválido'),
  password: z.string().min(1, 'Senha obrigatória'),
});

type LoginForm = z.infer<typeof loginSchema>;

export function Login() {
  const navigate = useNavigate();
  const location = useLocation();
  const { isAuthenticated, user, setToken } = useAuth();
  const loginMutation = useLogin();

  const from = (location.state as { from?: { pathname: string } } | null)?.from?.pathname ?? '/';

  // Se já autenticado (ex.: sessão restaurada via silent refresh), redirecionar
  // para o destino salvo ou para a home do papel — nunca para '/' genérico, que
  // mandaria o Vendedor por /dashboard/consolidated → /403.
  useEffect(() => {
    if (isAuthenticated) {
      const dest = from && from !== '/' ? from : homePathForRole(user?.role);
      navigate(dest, { replace: true });
    }
  }, [isAuthenticated, user, from, navigate]);

  const form = useForm<LoginForm>({
    schema: loginSchema,
    defaultValues: { email: '', password: '' },
  });

  async function onSubmit(data: LoginForm) {
    const result = await loginMutation.mutateAsync(data);
    setToken(result.accessToken);
    // Navegar role-aware a partir do token recém-emitido (não depende do estado
    // assíncrono do contexto), evitando a corrida que levava o Vendedor ao /403.
    const decoded = decodeJwtPayload(result.accessToken);
    const dest = from && from !== '/' ? from : homePathForRole(decoded?.role);
    navigate(dest, { replace: true });
  }

  const inputStyle = {
    width: '100%',
    padding: '10px 14px',
    borderRadius: '8px',
    border: '1px solid #45475a',
    background: '#313244',
    color: '#cdd6f4',
    fontSize: '0.95rem',
    outline: 'none',
    boxSizing: 'border-box' as const,
  };

  const labelStyle = {
    display: 'block',
    marginBottom: '6px',
    fontSize: '0.85rem',
    color: '#a6adc8',
    fontWeight: 500,
  };

  const errorStyle = {
    color: '#f38ba8',
    fontSize: '0.8rem',
    marginTop: '4px',
  };

  const linkStyle = {
    color: '#cba6f7',
    fontWeight: 600,
    textDecoration: 'underline',
  };

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      background: '#1e1e2e',
    }}>
      <div style={{
        background: '#181825',
        borderRadius: '16px',
        padding: '2.5rem',
        width: '100%',
        maxWidth: '400px',
        boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
      }}>
        <h1 style={{
          color: '#cba6f7',
          fontSize: '1.6rem',
          fontWeight: 700,
          marginBottom: '0.25rem',
          textAlign: 'center',
        }}>
          FinDash
        </h1>
        <p style={{ color: '#a6adc8', textAlign: 'center', marginBottom: '2rem', fontSize: '0.9rem' }}>
          Sistema de Comissões
        </p>

        <form onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div style={{ marginBottom: '1.25rem' }}>
            <label style={labelStyle} htmlFor="email">E-mail</label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              style={inputStyle}
              {...form.register('email')}
              aria-invalid={!!form.errors.email}
              aria-describedby={form.errors.email ? 'email-error' : undefined}
            />
            {form.errors.email && (
              <p id="email-error" style={errorStyle} role="alert">
                {form.errors.email}
              </p>
            )}
          </div>

          <div style={{ marginBottom: '1.75rem' }}>
            <label style={labelStyle} htmlFor="password">Senha</label>
            <input
              id="password"
              type="password"
              autoComplete="current-password"
              style={inputStyle}
              {...form.register('password')}
              aria-invalid={!!form.errors.password}
              aria-describedby={form.errors.password ? 'password-error' : undefined}
            />
            {form.errors.password && (
              <p id="password-error" style={errorStyle} role="alert">
                {form.errors.password}
              </p>
            )}
          </div>

          {/* Erro de API */}
          {loginMutation.error && (
            <div role="alert" style={{
              background: '#3b0000',
              border: '1px solid #f38ba8',
              borderRadius: '8px',
              padding: '10px 14px',
              marginBottom: '1.25rem',
              color: '#f38ba8',
              fontSize: '0.875rem',
            }}>
              {loginMutation.error instanceof Error
                ? (loginMutation.error.message.includes('401')
                  ? 'E-mail ou senha incorretos'
                  : loginMutation.error.message)
                : 'Erro ao fazer login'}
            </div>
          )}

          <button
            type="submit"
            disabled={loginMutation.isPending}
            style={{
              width: '100%',
              padding: '11px',
              borderRadius: '8px',
              border: 'none',
              background: '#cba6f7',
              color: '#1e1e2e',
              fontWeight: 700,
              fontSize: '0.95rem',
              cursor: loginMutation.isPending ? 'not-allowed' : 'pointer',
              opacity: loginMutation.isPending ? 0.7 : 1,
              transition: 'opacity 0.15s',
            }}
          >
            {loginMutation.isPending ? 'Entrando…' : 'Entrar'}
          </button>
        </form>

        {/* Disclaimer — prova de conceito gerada autonomamente (cstk + agente-00c) */}
        <div style={{
          marginTop: '1.75rem',
          paddingTop: '1.25rem',
          borderTop: '1px solid #313244',
          fontSize: '0.78rem',
          lineHeight: 1.55,
          color: '#a6adc8',
        }}>
          <p style={{ margin: 0 }}>
            <strong style={{ color: '#f9e2af' }}>⚠️ Prova de conceito.</strong>{' '}
            Projeto experimental — briefing, código, testes e documentação foram
            gerados de forma autônoma pelo{' '}
            <a
              href="https://github.com/JotJunior/cstk"
              target="_blank"
              rel="noreferrer"
              style={linkStyle}
            >
              cstk + agente-00c
            </a>
            . Não é software pronto para produção.
          </p>
          <p style={{ margin: '0.75rem 0 0' }}>
            🔑 As credenciais de teste (Gestor / Financeiro / Vendedor) estão no{' '}
            <a
              href="https://github.com/JotJunior/poc-financial-dashboard#-credenciais-de-demonstração"
              target="_blank"
              rel="noreferrer"
              style={linkStyle}
            >
              README do repositório
            </a>
            .
          </p>
          <p style={{ margin: '0.75rem 0 0' }}>
            Repositórios:{' '}
            <a
              href="https://github.com/JotJunior/poc-financial-dashboard"
              target="_blank"
              rel="noreferrer"
              style={linkStyle}
            >
              projeto
            </a>
            {' · '}
            <a
              href="https://github.com/JotJunior/cstk"
              target="_blank"
              rel="noreferrer"
              style={linkStyle}
            >
              cstk
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}
