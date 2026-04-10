package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func TestMemoryChatRepository(t *testing.T) {
	repo := NewChatRepository()
	chat := domain.Chat{ID: 123}
	ctx := context.Background()

	t.Run("Create Chat", func(t *testing.T) {
		err := repo.Create(ctx, chat)
		require.NoErrorf(t, err, "expected no error, got %v", err)
	})

	t.Run("Create Duplicate Chat", func(t *testing.T) {
		err := repo.Create(ctx, chat)
		require.Truef(t, errors.Is(err, domain.ErrChatAlreadyRegistered), "expected ErrChatAlreadyRegistered, got %v", err)
	})

	t.Run("Get Chat By Id", func(t *testing.T) {
		got, err := repo.GetById(ctx, 123)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Equalf(t, chat, got, "expected %v, got %v", chat, got)
	})

	t.Run("Delete Chat", func(t *testing.T) {
		err := repo.Delete(ctx, chat)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		_, err = repo.GetById(ctx, 123)
		require.Truef(t, errors.Is(err, domain.ErrChatNotRegistered), "expected ErrChatNotRegistered, got %v", err)
	})
}
