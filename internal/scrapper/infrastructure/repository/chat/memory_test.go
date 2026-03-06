package chat

import (
	"errors"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"testing"
)

func TestMemoryChatRepository(t *testing.T) {
	repo := NewMemoryRepository()
	chat := domain.Chat{ID: 123}

	t.Run("Create Chat", func(t *testing.T) {
		err := repo.Create(chat)
		require.NoErrorf(t, err, "expected no error, got %v", err)
	})

	t.Run("Create Duplicate Chat", func(t *testing.T) {
		err := repo.Create(chat)
		require.Truef(t, errors.Is(err, domain.ErrChatAlreadyRegistered), "expected ErrChatAlreadyRegistered, got %v", err)
	})

	t.Run("Get Chat By Id", func(t *testing.T) {
		got, err := repo.GetById(123)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Equalf(t, chat, got, "expected %v, got %v", chat, got)
	})

	t.Run("Delete Chat", func(t *testing.T) {
		err := repo.Delete(chat)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		_, err = repo.GetById(123)
		require.Truef(t, errors.Is(err, domain.ErrChatNotRegistered), "expected ErrChatNotRegistered, got %v", err)
	})
}
