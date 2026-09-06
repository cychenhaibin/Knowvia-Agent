package auth

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type TokenPair struct {
	AccessToken   string
	RefreshToken  string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	User          domain.User
	FluxASite     *FluxASite
	FluxAGroup    string
	FluxAUsername string
}
