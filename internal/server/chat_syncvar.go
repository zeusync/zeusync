package server

import (
	"github.com/zeusync/zeusync/internal/core/sync"
)

type ChatHistory struct {
	sync.BaseSyncVar[[]ChatMessage]
}

func NewChatHistory() *ChatHistory {
	return &ChatHistory{
		BaseSyncVar: *sync.NewBaseSyncVar[[]ChatMessage](make([]ChatMessage, 0)),
	}
}
