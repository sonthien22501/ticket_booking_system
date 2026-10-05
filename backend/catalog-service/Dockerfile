# Multi-stage Dockerfile for Event Catalog Service
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/catalog-service .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/catalog-service /app/catalog-service

USER appuser
EXPOSE 8081

HEALTHCHECK --interval=5s --timeout=3s --retries=5 --start-period=10s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

ENTRYPOINT ["/app/catalog-service"]
