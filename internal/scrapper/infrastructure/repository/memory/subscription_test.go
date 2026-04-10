package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

func TestMemorySubscriptionRepository(t *testing.T) {
	repo := NewSubscriptionRepository()
	sub1 := domain.Subscription{ChatID: 10, LinkID: 100, Tags: []string{"go"}}
	sub2 := domain.Subscription{ChatID: 20, LinkID: 100, Tags: []string{"golang"}}
	sub3 := domain.Subscription{ChatID: 10, LinkID: 200, Tags: []string{"java"}}
	ctx := context.Background()

	t.Run("Save Subscriptions", func(t *testing.T) {
		err := repo.Save(ctx, sub1)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
		err = repo.Save(ctx, sub2)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
		err = repo.Save(ctx, sub3)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
	})

	t.Run("Get By Chat Id", func(t *testing.T) {
		subs, err := repo.GetByChatId(ctx, 10)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Lenf(t, subs, 2, "expected 2 subscriptions for chat 10, got %d", len(subs))
	})

	t.Run("Get By Link Id", func(t *testing.T) {
		subs, err := repo.GetByLinkId(ctx, 100)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Lenf(t, subs, 2, "expected 2 subscriptions for chat 100, got %d", len(subs))
	})

	t.Run("Delete Subscription", func(t *testing.T) {
		sub, err := repo.Delete(ctx, sub2)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
		assert.Equalf(t, sub2.ChatID, sub.ChatID, "expected sub: %v, got: %v", sub2.ChatID, sub.ChatID)

		subs, err := repo.GetByLinkId(ctx, 100)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		assert.Lenf(t, subs, 1, "expected 1 subscription remaining, got %d", len(subs))

		sub, err = repo.Delete(ctx, sub1)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
		assert.Equalf(t, sub1, sub, "expected sub: %v, got: %v", sub1, sub)

		_, err = repo.GetByLinkId(ctx, 100)
		assert.Truef(t, errors.Is(err, domain.ErrLinkNotFound), "expected ErrLinkNotFound since all subs are deleted, got %v", err)
	})
}
