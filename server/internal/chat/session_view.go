package chat

import (
	"time"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type MessageSourceView struct {
	Provider     domain.Provider
	Title        string
	Repo         string
	URL          string
	Snippet      string
	MatchedLines []string
	Score        float64
}

type MessageView struct {
	ID           string
	SessionID    string
	Role         domain.ChatRole
	Content      string
	Skill        string
	UseKnowledge bool
	CreatedAt    time.Time
	CompletedAt  *time.Time
	Sources      []MessageSourceView
	Usage        *domain.ChatUsage
}
