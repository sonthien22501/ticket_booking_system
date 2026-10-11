FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download || true

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/auth-service .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /app/auth-service /app/auth-service

USER appuser
EXPOSE 8084

HEALTHCHECK --interval=5s --timeout=3s --retries=5 --start-period=5s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8084/health || exit 1

ENTRYPOINT ["/app/auth-service"]
