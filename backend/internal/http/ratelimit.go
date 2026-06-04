// Package http — rate limiter em memória para /auth/login.
// CHK — OWASP finding medium: max 5 tentativas por IP em 60s.
// Implementação: sliding window com sync.Mutex; adequado para instância única.
// Para deploy multi-instância, trocar por Redis (decisão de arquitetura futura).
package http

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimitEntry armazena as tentativas de um IP.
type rateLimitEntry struct {
	attempts  []time.Time
	mu        sync.Mutex
}

// RateLimiter implementa sliding-window rate limit por IP.
type RateLimiter struct {
	maxAttempts int
	window      time.Duration

	mu      sync.Mutex
	entries map[string]*rateLimitEntry
}

// NewRateLimiter cria um RateLimiter com os parâmetros fornecidos.
// maxAttempts=5, window=60s corresponde à exigência CHK/OWASP.
func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		entries:     make(map[string]*rateLimitEntry),
	}
	// Limpeza periódica de entradas antigas.
	go rl.cleanup()
	return rl
}

// Allow verifica se o IP pode fazer mais uma tentativa.
// Registra a tentativa e retorna true se dentro do limite.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	entry, ok := rl.entries[ip]
	if !ok {
		entry = &rateLimitEntry{}
		rl.entries[ip] = entry
	}
	rl.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Remover tentativas fora da janela.
	valid := entry.attempts[:0]
	for _, t := range entry.attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	entry.attempts = valid

	if len(entry.attempts) >= rl.maxAttempts {
		return false
	}

	entry.attempts = append(entry.attempts, now)
	return true
}

// cleanup remove entradas antigas a cada 5 minutos para evitar memory leak.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-rl.window)
		rl.mu.Lock()
		for ip, entry := range rl.entries {
			entry.mu.Lock()
			valid := entry.attempts[:0]
			for _, t := range entry.attempts {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			entry.attempts = valid
			if len(entry.attempts) == 0 {
				delete(rl.entries, ip)
			}
			entry.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// extractIP extrai o IP real da requisição (X-Real-IP → RemoteAddr).
func extractIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		// Pegar apenas o primeiro IP da lista.
		for i := 0; i < len(ip); i++ {
			if ip[i] == ',' {
				return ip[:i]
			}
		}
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
