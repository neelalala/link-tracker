package chat

import (
	"context"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"sync"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	chats map[int64]domain.Chat
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		chats: make(map[int64]domain.Chat),
	}
}

func (chatRepo *MemoryRepository) Create(ctx context.Context, chat domain.Chat) error {
	chatRepo.mu.Lock()
	defer chatRepo.mu.Unlock()
	if _, ok := chatRepo.chats[chat.ID]; ok {
		return domain.ErrChatAlreadyRegistered
	}
	chatRepo.chats[chat.ID] = chat
	return nil
}

func (chatRepo *MemoryRepository) GetById(ctx context.Context, id int64) (domain.Chat, error) {
	chatRepo.mu.RLock()
	defer chatRepo.mu.RUnlock()
	if chat, ok := chatRepo.chats[id]; ok {
		return chat, nil
	}
	return domain.Chat{}, domain.ErrChatNotRegistered
}

func (chatRepo *MemoryRepository) Delete(ctx context.Context, chat domain.Chat) error {
	chatRepo.mu.Lock()
	defer chatRepo.mu.Unlock()
	if _, ok := chatRepo.chats[chat.ID]; !ok {
		return domain.ErrChatNotRegistered
	}
	delete(chatRepo.chats, chat.ID)
	return nil
}
