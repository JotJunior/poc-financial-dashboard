# Multi-stage build para o frontend React/Vite — task 10.2.3
# Estágio 1: build do bundle Vite
FROM node:20-alpine AS builder

WORKDIR /app

# Build args para variáveis de ambiente Vite (injetadas em build-time)
ARG VITE_API_BASE_URL=/api/v1

# Copiar package.json primeiro (cache de camadas)
COPY package.json package-lock.json ./
RUN npm ci --prefer-offline

# Copiar código-fonte e buildar
COPY . .
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
RUN npm run build

# Estágio 2: nginx para servir static files
FROM nginx:1.27-alpine

# Copiar dist do Vite para nginx
COPY --from=builder /app/dist /usr/share/nginx/html

# Configuração nginx para SPA (React Router — fallback para index.html)
RUN cat > /etc/nginx/conf.d/default.conf << 'EOF'
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_types text/plain text/css application/json application/javascript;

    # SPA fallback: qualquer rota não-arquivo → index.html
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Cache para assets com hash (imutáveis)
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
EOF

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
    CMD wget -qO- http://localhost:80/ | grep -q html || exit 1
