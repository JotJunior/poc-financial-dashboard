package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"financial-dashboard/backend/internal/auth"
	apphttp "financial-dashboard/backend/internal/http"
	"financial-dashboard/backend/internal/http/middleware"
	"financial-dashboard/backend/internal/repository"
	"financial-dashboard/backend/internal/service"
)

func main() {
	// Configuração via environment
	port := getEnv("SERVER_PORT", "8080")
	tlsCert := os.Getenv("TLS_CERT_PATH")
	tlsKey := os.Getenv("TLS_KEY_PATH")
	dbURL := getEnv("DATABASE_URL", "postgres://financialuser:financialpass@localhost:5433/financial_dashboard?sslmode=disable")

	// Logger estruturado (constitution P-I — auditabilidade)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Conexão com PostgreSQL
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("falha ao criar pool pgx", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("falha ao conectar ao PostgreSQL", "err", err, "url", dbURL)
		os.Exit(1)
	}
	slog.Info("PostgreSQL conectado")

	// Dependências
	userRepo := repository.NewUserRepository(pool)
	userAdapter := apphttp.NewUserAdapter(userRepo)
	blocklist := auth.NewDBBlocklist(pool)

	// Rate limiter: 5 tentativas / IP / 60s (CHK / OWASP finding medium)
	rateLimiter := apphttp.NewRateLimiter(5, 60*time.Second)

	authHandler := apphttp.NewAuthHandler(userAdapter, blocklist, rateLimiter)

	// Vendor dependencies
	vendorRepo := repository.NewPGVendorRepository(pool)
	commissionRuleRepo := repository.NewPGCommissionRuleRepository(pool)
	vendorSvc := service.NewVendorService(vendorRepo, commissionRuleRepo, nil)
	vendorHandler := apphttp.NewVendorHandler(vendorSvc)

	// Order dependencies (FASE 4)
	orderRepo := repository.NewPGOrderRepository(pool)
	commissionRepo := repository.NewPGCommissionRepository(pool)
	orderSvc := service.NewOrderService(orderRepo, commissionRepo, commissionRuleRepo)
	orderHandler := apphttp.NewOrderHandler(orderSvc)

	// Verifier que combina VerifyToken + blocklist (para RequireAuth).
	authVerifier := func(r *http.Request, tokenString string) (*auth.Claims, error) {
		return auth.VerifyTokenWithBlocklist(r.Context(), tokenString, blocklist)
	}

	// Router chi
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	})

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth — rotas públicas (sem RequireAuth)
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/refresh", authHandler.Refresh)

		// Auth — logout requer token válido
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(authVerifier))
			r.Post("/auth/logout", authHandler.Logout)
		})

		// Rotas protegidas — FASE 3: Vendedores
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(authVerifier))

			// GET /vendors — Gestor e Financeiro
			r.With(middleware.RequireRole("gestor", "financeiro")).Get("/vendors", vendorHandler.List)

			// POST /vendors — apenas Gestor
			r.With(middleware.RequireRole("gestor")).Post("/vendors", vendorHandler.Create)

			// GET /vendors/{id} — Gestor, Financeiro e Vendedor (com scope)
			r.Get("/vendors/{id}", vendorHandler.Get)

			// PATCH /vendors/{id} — apenas Gestor
			r.With(middleware.RequireRole("gestor")).Patch("/vendors/{id}", vendorHandler.Update)

			// DELETE /vendors/{id} — apenas Gestor (anonimização LGPD)
			r.With(middleware.RequireRole("gestor")).Delete("/vendors/{id}", vendorHandler.Delete)

			// FASE 4: Pedidos
			// GET /orders — todos os papéis (Vendedor com escopo)
			r.Get("/orders", orderHandler.List)
			// POST /orders — apenas Gestor
			r.With(middleware.RequireRole("gestor")).Post("/orders", orderHandler.Create)
			// GET /orders/{id} — todos os papéis
			r.Get("/orders/{id}", orderHandler.Get)
			// PATCH /orders/{id}/status — apenas Gestor
			r.With(middleware.RequireRole("gestor")).Patch("/orders/{id}/status", orderHandler.Transition)
		})
	})

	// Servidor HTTP
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if tlsCert != "" && tlsKey != "" {
			slog.Info("starting HTTPS server", "addr", srv.Addr)
			srv.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
			if err := srv.ListenAndServeTLS(tlsCert, tlsKey); err != nil && err != http.ErrServerClosed {
				slog.Error("server error", "err", err)
				os.Exit(1)
			}
		} else {
			slog.Info("starting HTTP server (dev mode — use TLS_CERT_PATH/TLS_KEY_PATH for production)", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("server error", "err", err)
				os.Exit(1)
			}
		}
	}()

	<-shutdownCtx.Done()
	slog.Info("shutdown signal received")

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(timeoutCtx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
