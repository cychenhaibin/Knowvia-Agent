package app

import (
	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/mirror"
)

func buildMirrorService(mirrorStore mirror.MirrorStore, clients externalClients) *mirror.Service {
	return mirror.NewService(mirrorStore, clients.forward)
}
