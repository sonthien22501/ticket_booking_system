# Multi-stage Dockerfile for Seat Inventory Service
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/inventory-service .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/inventory-service /app/inventory-service

USER appuser
EXPOSE 8082

HEALTHCHECK --interval=5s --timeout=3s --retries=5 --start-period=10s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

ENTRYPOINT ["/app/inventory-service"]
