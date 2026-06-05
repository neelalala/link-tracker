package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/valkey-io/valkey-go"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type cachedLink struct {
	ID   int64    `json:"id"`
	URL  string   `json:"url"`
	Tags []string `json:"tags"`
}

type Cache struct {
	client            valkey.Client
	ttl               time.Duration
	keyPrefix         string
	clientSideCaching bool
}

func New(
	addresses []string,
	username, password string,
	ttl time.Duration,
	keyPrefix string,
	clientSideCaching bool,
) (*Cache, error) {
	client, err := valkey.NewClient(
		valkey.ClientOption{
			InitAddress:  addresses,
			Username:     username,
			Password:     password,
			DisableCache: !clientSideCaching,
		})
	if err != nil {
		return nil, fmt.Errorf("valkey cache: failed to create client: %w", err)
	}
	return &Cache{
		client:            client,
		ttl:               ttl,
		keyPrefix:         keyPrefix,
		clientSideCaching: clientSideCaching,
	}, nil
}

func (cache *Cache) key(chatID int64) string {
	return cache.keyPrefix + strconv.FormatInt(chatID, 10)
}

func (cache *Cache) Close() error {
	if cache.client != nil {
		cache.client.Close()
	}
	return nil
}

func (cache *Cache) GetLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, bool, error) {
	key := cache.key(chatID)

	var resp valkey.ValkeyResult
	if cache.clientSideCaching {
		resp = cache.client.DoCache(ctx, cache.client.B().Get().Key(key).Cache(), cache.ttl)
	} else {
		resp = cache.client.Do(ctx, cache.client.B().Get().Key(key).Build())
	}

	if err := resp.Error(); err != nil {
		return nil, false, fmt.Errorf("error getting cache entry: %w", err)
	}

	body, err := resp.AsBytes()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("error reading cache body as bytes: %w", err)
	}

	var cachedLinks []cachedLink
	if err := json.Unmarshal(body, &cachedLinks); err != nil {
		return nil, false, fmt.Errorf("error unmarshalling cached links: %w", err)
	}

	links := make([]domain.TrackedLink, len(cachedLinks))
	for i, link := range cachedLinks {
		links[i] = domain.TrackedLink{
			ID:   link.ID,
			URL:  link.URL,
			Tags: link.Tags,
		}
	}

	return links, true, nil
}

func (cache *Cache) SetLinks(ctx context.Context, chatID int64, links []domain.TrackedLink) error {
	key := cache.key(chatID)

	cachedLinks := make([]cachedLink, len(links))
	for i, link := range links {
		cachedLinks[i] = cachedLink{
			ID:   link.ID,
			URL:  link.URL,
			Tags: link.Tags,
		}
	}

	body, err := json.Marshal(cachedLinks)
	if err != nil {
		return fmt.Errorf("error marshalling cached links: %w", err)
	}

	if err := cache.client.Do(
		ctx,
		cache.client.B().Set().Key(key).Value(string(body)).Ex(cache.ttl).Build(),
	).Error(); err != nil {
		return fmt.Errorf("error setting cache entry for %s: %w", key, err)
	}

	return nil
}

func (cache *Cache) Invalidate(ctx context.Context, chatID int64) error {
	key := cache.key(chatID)

	if err := cache.client.Do(ctx, cache.client.B().Del().Key(key).Build()).Error(); err != nil {
		return fmt.Errorf("error invalidating cache entry: %w", err)
	}

	return nil
}
