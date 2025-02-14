FROM golang:1.24-alpine3.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /orchestrator

FROM alpine:3.18
RUN apk --no-cache add ca-certificates curl wget
WORKDIR /app
COPY --from=builder /orchestrator /app/orchestrator
COPY scripts/register-service.sh /app/register-service.sh
RUN chmod +x /app/orchestrator /app/register-service.sh
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost/health || exit 1
ENTRYPOINT ["/bin/sh", "-c", "/app/orchestrator & /app/register-service.sh & wait"]