package subscription

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"testing"
)

func TestMemorySubscriptionRepository(t *testing.T) {
	repo := NewMemoryRepository()
	sub1 := domain.Subscription{ChatID: 10, LinkID: 100, Tags: []string{"go"}}
	sub2 := domain.Subscription{ChatID: 20, LinkID: 100, Tags: []string{"golang"}}
	sub3 := domain.Subscription{ChatID: 10, LinkID: 200, Tags: []string{"java"}}

	t.Run("Save Subscriptions", func(t *testing.T) {
		err := repo.Save(sub1)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
		err = repo.Save(sub2)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
		err = repo.Save(sub3)
		assert.NoErrorf(t, err, "expected no error, got %v", err)
	})

	t.Run("Get By Chat Id", func(t *testing.T) {
		subs, err := repo.GetByChatId(10)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Lenf(t, subs, 2, "expected 2 subscriptions for chat 10, got %d", len(subs))
	})

	t.Run("Get By Link Id", func(t *testing.T) {
		subs, err := repo.GetByLinkId(100)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Lenf(t, subs, 2, "expected 2 subscriptions for chat 100, got %d", len(subs))
	})

	t.Run("Delete Subscription", func(t *testing.T) {
		err := repo.Delete(sub2)
		assert.NoErrorf(t, err, "expected no error, got %v", err)

		subs, err := repo.GetByLinkId(100)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		assert.Lenf(t, subs, 1, "expected 1 subscription remaining, got %d", len(subs))

		err = repo.Delete(sub1)
		assert.NoErrorf(t, err, "expected no error, got %v", err)

		_, err = repo.GetByLinkId(100)
		assert.Truef(t, errors.Is(err, domain.ErrLinkNotFound), "expected ErrLinkNotFound since all subs are deleted, got %v", err)
	})
}
