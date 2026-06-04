# Multi-stage build para o backend Go — task 10.2.3
# Estágio 1: build do binário estático
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copiar módulos primeiro (cache de camadas)
COPY go.mod go.sum ./
RUN go mod download

# Copiar código-fonte
COPY . .

# Build do binário estático (CGO desabilitado para distroless/alpine mínima)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app/api ./cmd/api/

# Estágio 2: imagem final mínima
FROM alpine:3.20

# Instalar ca-certificates para conexões TLS/HTTPS externas (pool PostgreSQL)
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copiar apenas o binário compilado
COPY --from=builder /app/api /app/api

# Usuário não-root (CHK025 — princípio do menor privilégio)
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/ready | grep -q ready || exit 1

ENTRYPOINT ["/app/api"]
