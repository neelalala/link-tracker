package application

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type MockLinkRepo struct {
	GetBatchFunc func(limit, offset int) ([]domain.Link, error)
	GetByIdFunc  func(id int64) (domain.Link, error)
	GetByUrlFunc func(url string) (domain.Link, error)
	SaveFunc     func(link domain.Link) (domain.Link, error)
	DeleteFunc   func(link domain.Link) error
}

func (m *MockLinkRepo) GetBatch(limit, offset int) ([]domain.Link, error) {
	if m.GetBatchFunc != nil {
		return m.GetBatchFunc(limit, offset)
	}
	return nil, nil
}

func (m *MockLinkRepo) GetById(id int64) (domain.Link, error) {
	if m.GetByIdFunc != nil {
		return m.GetByIdFunc(id)
	}
	return domain.Link{}, nil
}

func (m *MockLinkRepo) GetByUrl(url string) (domain.Link, error) {
	if m.GetByUrlFunc != nil {
		return m.GetByUrlFunc(url)
	}
	return domain.Link{}, nil
}

func (m *MockLinkRepo) Save(link domain.Link) (domain.Link, error) {
	if m.SaveFunc != nil {
		return m.SaveFunc(link)
	}
	return link, nil
}

func (m *MockLinkRepo) Delete(link domain.Link) error {
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

func (m *MockSubRepo) GetByLinkId(linkID int64) ([]domain.Subscription, error) {
	if m.GetByLinkIdFunc != nil {
		return m.GetByLinkIdFunc(linkID)
	}
	return nil, nil
}

func (m *MockSubRepo) GetByChatId(chatID int64) ([]domain.Subscription, error) {
	if m.GetByChatIdFunc != nil {
		return m.GetByChatIdFunc(chatID)
	}
	return nil, nil
}

func (m *MockSubRepo) Save(sub domain.Subscription) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(sub)
	}
	return nil
}

func (m *MockSubRepo) Delete(sub domain.Subscription) (domain.Subscription, error) {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(sub)
	}
	return sub, nil
}

type MockNotifier struct {
	SentUpdates []domain.LinkUpdate
}

func (m *MockNotifier) SendUpdate(update domain.LinkUpdate) error {
	m.SentUpdates = append(m.SentUpdates, update)
	return nil
}

type MockFetcher struct{}

func (f *MockFetcher) CanHandle(string) bool { return true }

func (f *MockFetcher) Fetch(string) (FetchResult, error) {
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

	service.processLink(testLink)

	require.Lenf(t, notifier.SentUpdates, 1, "Expected 1 update sent, got %d", len(notifier.SentUpdates))

	update := notifier.SentUpdates[0]

	expectedChatIDs := []int64{100, 200}

	assert.Lenf(t, update.TgChatIDs, len(expectedChatIDs), "Expected %d recipients, got %d", len(expectedChatIDs), len(update.TgChatIDs))

	assert.Equalf(t, update.TgChatIDs, expectedChatIDs, "Expected recipients %v, got %v", expectedChatIDs, update.TgChatIDs)
}
