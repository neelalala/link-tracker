package link

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"testing"
)

func TestMemoryLinkRepository(t *testing.T) {
	repo := NewMemoryRepository()
	link := domain.Link{ID: 1, URL: "https://example.com/"}
	ctx := context.Background()

	t.Run("Save Link", func(t *testing.T) {
		savedLink, err := repo.Save(ctx, link)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		assert.Equalf(t, savedLink.URL, link.URL, "expected %v, got %v", link.URL, savedLink.URL)
	})

	t.Run("Get Link By Url", func(t *testing.T) {
		got, err := repo.GetByUrl(ctx, "https://example.com/")
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Equalf(t, link, got, "expected %v, got %v", link, got)
	})

	t.Run("Get Link By Id", func(t *testing.T) {
		got, err := repo.GetById(ctx, 1)
		require.NoErrorf(t, err, "expected no error, got %v", err)
		require.Equalf(t, link, got, "expected %v, got %v", link, got)
	})

	t.Run("Delete Link", func(t *testing.T) {
		err := repo.Delete(ctx, link)
		require.NoErrorf(t, err, "expected no error, got %v", err)

		_, err = repo.GetByUrl(ctx, link.URL)
		require.Truef(t, errors.Is(err, domain.ErrLinkNotFound), "expected ErrLinkNotFound, got %v", err)
	})
}
