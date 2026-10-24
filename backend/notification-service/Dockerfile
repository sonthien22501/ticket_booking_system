# Multi-stage Dockerfile for Notification Worker Microservice
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/notification-service .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/notification-service /app/notification-service

USER appuser
EXPOSE 8085

HEALTHCHECK --interval=5s --timeout=3s --retries=5 --start-period=10s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8085/health || exit 1

ENTRYPOINT ["/app/notification-service"]
