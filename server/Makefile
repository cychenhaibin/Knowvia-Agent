APP_NAME=quickque-agent

.PHONY: api api-memory api-postgres worker test test-core

api:
	go run ./cmd/api

api-memory:
	QQA_STORE_BACKEND=memory go run ./cmd/api

api-postgres:
	QQA_STORE_BACKEND=postgres go run ./cmd/api

worker:
	go run ./cmd/worker

test:
	go test ./...

test-core:
	go test ./internal/run ./internal/knowledge
