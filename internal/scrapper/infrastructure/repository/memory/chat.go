package memory

import (
	"context"
	"sync"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type ChatRepository struct {
	mu    sync.RWMutex
	chats map[int64]domain.Chat
}

func NewChatRepository() *ChatRepository {
	return &ChatRepository{
		chats: make(map[int64]domain.Chat),
	}
}

func (chatRepo *ChatRepository) Create(ctx context.Context, chat domain.Chat) error {
	chatRepo.mu.Lock()
	defer chatRepo.mu.Unlock()
	if _, ok := chatRepo.chats[chat.ID]; ok {
		return domain.ErrChatAlreadyRegistered
	}
	chatRepo.chats[chat.ID] = chat
	return nil
}

func (chatRepo *ChatRepository) GetById(ctx context.Context, id int64) (domain.Chat, error) {
	chatRepo.mu.RLock()
	defer chatRepo.mu.RUnlock()
	if chat, ok := chatRepo.chats[id]; ok {
		return chat, nil
	}
	return domain.Chat{}, domain.ErrChatNotRegistered
}

func (chatRepo *ChatRepository) Delete(ctx context.Context, chat domain.Chat) error {
	chatRepo.mu.Lock()
	defer chatRepo.mu.Unlock()
	if _, ok := chatRepo.chats[chat.ID]; !ok {
		return domain.ErrChatNotRegistered
	}
	delete(chatRepo.chats, chat.ID)
	return nil
}
