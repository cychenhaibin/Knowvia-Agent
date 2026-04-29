package provider

import (
	"context"

	"github.com/chenhaibin/yuque-rag/quickque-agent/server/internal/domain"
)

type ChatClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type ModelSelectableChatClient interface {
	CompleteWithConfig(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		runtime domain.ChatRuntimeConfig,
	) (string, error)
	StreamWithConfig(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		runtime domain.ChatRuntimeConfig,
		onDelta func(string) error,
		onUsage func(domain.ChatUsage) error,
	) (string, error)
}

type StreamingChatClient interface {
	ChatClient
	Stream(
		ctx context.Context,
		systemPrompt string,
		userPrompt string,
		onDelta func(string) error,
	) (string, error)
}
