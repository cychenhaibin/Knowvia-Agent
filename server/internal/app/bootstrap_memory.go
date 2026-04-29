package app

import (
	memorybackend "github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/adapters/store/memory"
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/config"
)

func bootstrapMemoryServices(cfg config.Config) (*Services, error) {
	memoryStore := memorybackend.New()
	return bootstrapServices(
		cfg,
		func() {},
		memoryStore,
		memoryStore,
		memoryStore,
		memoryStore,
		memoryStore,
		memoryStore,
		memoryStore,
		memoryStore,
	), nil
}
