module github.com/chenhaibin/yuque-rag/quickque-agent/server

go 1.25.0

require (
	github.com/alicebob/miniredis/v2 v2.37.0
	github.com/go-chi/chi/v5 v5.2.1
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.9.2
	github.com/joho/godotenv v1.5.1
	github.com/pgvector/pgvector-go v0.3.0
	github.com/redis/go-redis/v9 v9.18.0
	golang.org/x/crypto v0.36.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)

replace golang.org/x/sync => golang.org/x/sync v0.18.0

replace golang.org/x/text => golang.org/x/text v0.31.0

replace github.com/jackc/pgservicefile => github.com/jackc/pgservicefile v0.0.0-20231201235250-de7065d80cb9
