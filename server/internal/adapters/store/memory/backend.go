package memory

import basestore "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store"

type Store = basestore.MemoryStore

func New() *Store {
	return basestore.NewMemoryStore()
}
