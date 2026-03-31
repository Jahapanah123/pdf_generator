FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /worker ./cmd/worker

FROM alpine:3.19 AS api
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -g '' appuser
WORKDIR /app
COPY --from=builder /api .
RUN mkdir -p /app/output && chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
ENTRYPOINT ["./api"]

FROM alpine:3.19 AS worker
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -g '' appuser
WORKDIR /app
COPY --from=builder /worker .
RUN mkdir -p /app/output && chown -R appuser:appuser /app
USER appuser
ENTRYPOINT ["./worker"]