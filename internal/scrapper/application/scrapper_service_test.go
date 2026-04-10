package application

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type MockLinkRepo struct {
	GetBatchFunc func(limit, offset int) ([]domain.Link, error)
	GetByIdFunc  func(id int64) (domain.Link, error)
	GetByUrlFunc func(url string) (domain.Link, error)
	SaveFunc     func(link domain.Link) (domain.Link, error)
	DeleteFunc   func(link domain.Link) error
}

func (m *MockLinkRepo) GetBatch(ctx context.Context, limit, offset int) ([]domain.Link, error) {
	if m.GetBatchFunc != nil {
		return m.GetBatchFunc(limit, offset)
	}
	return nil, nil
}

func (m *MockLinkRepo) GetById(ctx context.Context, id int64) (domain.Link, error) {
	if m.GetByIdFunc != nil {
		return m.GetByIdFunc(id)
	}
	return domain.Link{}, nil
}

func (m *MockLinkRepo) GetByUrl(ctx context.Context, url string) (domain.Link, error) {
	if m.GetByUrlFunc != nil {
		return m.GetByUrlFunc(url)
	}
	return domain.Link{}, nil
}

func (m *MockLinkRepo) Save(ctx context.Context, link domain.Link) (domain.Link, error) {
	if m.SaveFunc != nil {
		return m.SaveFunc(link)
	}
	return link, nil
}

func (m *MockLinkRepo) Delete(ctx context.Context, link domain.Link) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(link)
	}
	return nil
}

type MockSubRepo struct {
	GetByLinkIdFunc func(linkID int64) ([]domain.Subscription, error)
	GetByChatIdFunc func(chatID int64) ([]domain.Subscription, error)
	SaveFunc        func(sub domain.Subscription) error
	DeleteFunc      func(sub domain.Subscription) (domain.Subscription, error)
}

func (m *MockSubRepo) GetByLinkId(ctx context.Context, linkID int64) ([]domain.Subscription, error) {
	if m.GetByLinkIdFunc != nil {
		return m.GetByLinkIdFunc(linkID)
	}
	return nil, nil
}

func (m *MockSubRepo) GetByChatId(ctx context.Context, chatID int64) ([]domain.Subscription, error) {
	if m.GetByChatIdFunc != nil {
		return m.GetByChatIdFunc(chatID)
	}
	return nil, nil
}

func (m *MockSubRepo) Exists(ctx context.Context, chatId int64, linkId int64) (bool, error) {
	return false, nil
}

func (m *MockSubRepo) Save(ctx context.Context, sub domain.Subscription) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(sub)
	}
	return nil
}

func (m *MockSubRepo) Delete(ctx context.Context, sub domain.Subscription) (domain.Subscription, error) {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(sub)
	}
	return sub, nil
}

func (m *MockSubRepo) AddTags(ctx context.Context, linkId, chatId int64, tags []string) error {
	return nil
}

func (m *MockSubRepo) GetTags(ctx context.Context, linkId, chatId int64) ([]string, error) {
	return []string{}, nil
}

func (m *MockSubRepo) DeleteTags(ctx context.Context, linkId, chatId int64, tags []string) error {
	return nil
}

type MockNotifier struct {
	SentUpdates []domain.LinkUpdate
}

func (m *MockNotifier) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	m.SentUpdates = append(m.SentUpdates, update)
	return nil
}

type MockFetcher struct{}

func (f *MockFetcher) CanHandle(string) bool { return true }

func (f *MockFetcher) Fetch(context.Context, string) (FetchResult, error) {
	return FetchResult{
		UpdatedAt:   time.Now().Add(1 * time.Hour),
		Description: "Mock update",
	}, nil
}

func logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestScrapperService_ProcessLink_NotifiesOnlySubscribers(t *testing.T) {
	notifier := &MockNotifier{}
	fetcherService := NewFetcherService([]LinkFetcher{&MockFetcher{}})

	mockSubRepo := &MockSubRepo{
		GetByLinkIdFunc: func(linkID int64) ([]domain.Subscription, error) {
			return []domain.Subscription{
				{ChatID: 100, LinkID: linkID},
				{ChatID: 200, LinkID: linkID},
			}, nil
		},
	}

	mockLinkRepo := &MockLinkRepo{
		SaveFunc: func(link domain.Link) (domain.Link, error) {
			return link, nil
		},
	}

	service := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, logger())

	testLink := domain.Link{
		ID:          1,
		URL:         "https://github.com/user/repo",
		LastUpdated: time.Now().Add(-1 * time.Hour),
	}

	service.processLink(context.Background(), testLink)

	require.Lenf(t, notifier.SentUpdates, 1, "Expected 1 update sent, got %d", len(notifier.SentUpdates))

	update := notifier.SentUpdates[0]

	expectedChatIDs := []int64{100, 200}

	assert.Lenf(t, update.TgChatIDs, len(expectedChatIDs), "Expected %d recipients, got %d", len(expectedChatIDs), len(update.TgChatIDs))

	assert.Equalf(t, update.TgChatIDs, expectedChatIDs, "Expected recipients %v, got %v", expectedChatIDs, update.TgChatIDs)
}
