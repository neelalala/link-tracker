package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/mocks"
)

type MockNotifier struct {
	SentUpdates []domain.LinkUpdate
}

func (m *MockNotifier) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	m.SentUpdates = append(m.SentUpdates, update)
	return nil
}

type TestUpdateEvent struct {
	Time time.Time
	Desc string
	Prev string
}

func (e TestUpdateEvent) UpdatedAt() time.Time { return e.Time }
func (e TestUpdateEvent) Description() string  { return e.Desc }
func (e TestUpdateEvent) Preview() string      { return e.Prev }

func logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestScrapperService_ProcessLink_NotifiesOnlySubscribers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockFetcher := mocks.NewMockLinkFetcher(ctrl)

	notifier := &MockNotifier{}

	testLink := domain.Link{
		ID:          1,
		URL:         "https://github.com/user/repo",
		LastUpdated: time.Now().Add(-1 * time.Hour),
	}

	mockEvent := TestUpdateEvent{
		Time: time.Now(),
		Desc: "New update",
		Prev: "Update preview",
	}

	mockFetcher.EXPECT().CanHandle(testLink.URL).Return(true).AnyTimes()

	mockFetcher.EXPECT().
		Fetch(gomock.Any(), testLink.URL, testLink.LastUpdated).
		Return([]domain.UpdateEvent{mockEvent}, nil)

	mockSubRepo.EXPECT().
		GetByLinkId(gomock.Any(), testLink.ID).
		Return([]domain.Subscription{
			{ChatID: 100, LinkID: testLink.ID},
			{ChatID: 200, LinkID: testLink.ID},
		}, nil)

	mockLinkRepo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(testLink, nil)

	fetcherService := NewFetcherService([]domain.LinkFetcher{mockFetcher})
	service := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, logger())

	service.processLink(context.Background(), testLink)

	require.Len(t, notifier.SentUpdates, 1, "Expected 1 update sent")

	update := notifier.SentUpdates[0]
	expectedChatIDs := []int64{100, 200}

	assert.Len(t, update.TgChatIDs, len(expectedChatIDs))
	assert.ElementsMatch(t, expectedChatIDs, update.TgChatIDs, "Chat IDs should match regardless of order")

	assert.Equal(t, testLink.URL, update.URL)
	assert.Equal(t, testLink.ID, update.ID)
}

func TestScrapperService_ProcessLink_FetcherError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockFetcher := mocks.NewMockLinkFetcher(ctrl)

	testLink := domain.Link{
		ID:          1,
		URL:         "https://github.com/user/repo",
		LastUpdated: time.Now().Add(-1 * time.Hour),
	}

	mockSubRepo.EXPECT().
		GetByLinkId(gomock.Any(), testLink.ID).
		Return([]domain.Subscription{{ChatID: 100, LinkID: testLink.ID}}, nil)

	notifier := &MockNotifier{}

	mockFetcher.EXPECT().CanHandle(testLink.URL).Return(true).AnyTimes()

	expectedErr := errors.New("github api timeout")
	mockFetcher.EXPECT().
		Fetch(gomock.Any(), testLink.URL, testLink.LastUpdated).
		Return(nil, expectedErr)

	fetcherService := NewFetcherService([]domain.LinkFetcher{mockFetcher})
	service := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, logger())

	service.processLink(context.Background(), testLink)

	assert.Empty(t, notifier.SentUpdates, "Expected no updates to be sent when fetcher fails")
}
