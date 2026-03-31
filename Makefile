.PHONY: build run-api run-worker docker-up docker-down test lint

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

run-api:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down -v

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run ./...

migrate-up:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/pdfgen?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/pdfgen?sslmode=disable" down