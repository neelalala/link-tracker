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
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/mocks"
	"go.uber.org/mock/gomock"
)

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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	notifier := &MockNotifier{}
	fetcherService := NewFetcherService([]LinkFetcher{&MockFetcher{}})

	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)

	testLink := domain.Link{
		ID:          1,
		URL:         "https://github.com/user/repo",
		LastUpdated: time.Now().Add(-1 * time.Hour),
	}

	subs := []domain.Subscription{
		{ChatID: 100, LinkID: testLink.ID},
		{ChatID: 200, LinkID: testLink.ID},
	}

	mockSubRepo.EXPECT().
		GetByLinkId(gomock.Any(), testLink.ID).
		Return(subs, nil).
		Times(1)

	mockLinkRepo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, link domain.Link) (domain.Link, error) {
			return link, nil
		}).
		Times(1)

	service := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, logger())

	service.processLink(context.Background(), testLink)

	require.Len(t, notifier.SentUpdates, 1)

	update := notifier.SentUpdates[0]
	expectedChatIDs := []int64{100, 200}

	assert.Len(t, update.TgChatIDs, len(expectedChatIDs))
	assert.Equal(t, expectedChatIDs, update.TgChatIDs)
}
