APP_NAME=quickque-agent

.PHONY: api api-memory api-postgres worker test test-core test-store-contract test-store-contract-postgres

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

test-store-contract:
	go test ./internal/adapters/store -run TestStoreContract

test-store-contract-postgres:
	@if [ -z "$$QQA_TEST_POSTGRES_DSN" ]; then \
		echo "QQA_TEST_POSTGRES_DSN is required"; \
		exit 1; \
	fi
	go test ./internal/adapters/store -run TestStoreContract
