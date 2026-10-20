# =============================================================================
# Backend — Go-сервис (строки: stdlib net/http, pgxpool, go-redis, gopdf).
# Многоэтапная сборка: компиляция в builder, запуск в slim-контейнере.
# =============================================================================
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Зависимости модулей кэшируются отдельно от исходников
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Сборка статического бинарника без CGO (минимальный размер образа)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags '-s -w' -o /server ./cmd/server

# Запуск в минимальном образе (нет даже shell, только бинарник)
FROM alpine:3.21

# CA-сертификаты для возможных исходящих HTTPS-запросов (например, Gesundheitscheck)
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /server /server

EXPOSE 8080

# Gesundheit: контейнер считается готовым, когда отвечает на /api/health
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/health || exit 1

ENTRYPOINT ["/server"]
