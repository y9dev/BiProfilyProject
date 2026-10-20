# =============================================================================
# Frontend — React + Vite, отдаётся через nginx.
# Этап 1: сборка статики (vite build).
# Этап 2: nginx с готовой статикой.
# =============================================================================
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# VITE_API_URL передаётся из docker-compose (http://backend:8080 внутри сети)
ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=${VITE_API_URL}

RUN npm run build

FROM nginx:1.27-alpine

# Готовая статика из этапа сборки
COPY --from=builder /app/dist /usr/share/nginx/html

# Конфиг nginx: SPA fallback (hash-роуты) + заголовки безопасности
RUN rm -f /etc/nginx/conf.d/default.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost/ || exit 1

CMD ["nginx", "-g", "daemon off;"]
