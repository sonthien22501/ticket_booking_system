FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
# COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service

FROM alpine:latest

WORKDIR /

COPY --from=builder /auth-service /auth-service

EXPOSE 8084

ENTRYPOINT ["/auth-service"]
