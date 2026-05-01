package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/mocks"
)

type MockNotifier struct {
	mu          sync.Mutex
	SentUpdates []domain.LinkUpdate
}

func (m *MockNotifier) SendUpdate(ctx context.Context, update domain.LinkUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func newLogger() *slog.Logger {
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
		GetByLinkID(gomock.Any(), testLink.ID).
		Return([]domain.Subscription{
			{ChatID: 100, LinkID: testLink.ID},
			{ChatID: 200, LinkID: testLink.ID},
		}, nil)

	mockLinkRepo.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(testLink, nil)

	fetcherService := NewFetcherService([]domain.LinkFetcher{mockFetcher})
	service, err := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, 100, 4, newLogger())
	require.NoError(t, err, "Expected no error on creating scrapper serivce")

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
		GetByLinkID(gomock.Any(), testLink.ID).
		Return([]domain.Subscription{{ChatID: 100, LinkID: testLink.ID}}, nil)

	notifier := &MockNotifier{}

	mockFetcher.EXPECT().CanHandle(testLink.URL).Return(true).AnyTimes()

	expectedErr := errors.New("github api timeout")
	mockFetcher.EXPECT().
		Fetch(gomock.Any(), testLink.URL, testLink.LastUpdated).
		Return(nil, expectedErr)

	fetcherService := NewFetcherService([]domain.LinkFetcher{mockFetcher})
	service, err := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, 100, 4, newLogger())
	require.NoError(t, err, "Expected no error on creation scrapper service")

	service.processLink(context.Background(), testLink)

	require.GreaterOrEqualf(t, len(notifier.SentUpdates), 1, "Expected 1 update sent")
	assert.Contains(t, notifier.SentUpdates[0].Description, "couldn't fetch your link")
}

func TestScrapperService_GetUpdates_BatchProcessedCorrectly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockFetcher := mocks.NewMockLinkFetcher(ctrl)

	notifier := &MockNotifier{}
	batchSize := 2

	linksBatch1 := []domain.Link{
		{ID: 1, URL: "https://github.com/user/repo1"},
		{ID: 2, URL: "https://github.com/user/repo2"},
	}
	linksBatch2 := []domain.Link{
		{ID: 3, URL: "https://stackoverflow.com/q/123"},
	}

	mockLinkRepo.EXPECT().GetBatch(gomock.Any(), batchSize, 0).Return(linksBatch1, nil)
	mockLinkRepo.EXPECT().GetBatch(gomock.Any(), batchSize, 2).Return(linksBatch2, nil)
	mockLinkRepo.EXPECT().GetBatch(gomock.Any(), batchSize, 4).Return([]domain.Link{}, nil)

	mockFetcher.EXPECT().CanHandle(gomock.Any()).Return(true).AnyTimes()

	allLinks := append(linksBatch1, linksBatch2...)
	for _, link := range allLinks {
		mockSubRepo.EXPECT().
			GetByLinkID(gomock.Any(), link.ID).
			Return([]domain.Subscription{{ChatID: 100}}, nil).AnyTimes()

		mockEvent := TestUpdateEvent{Time: time.Now(), Desc: "Update for " + link.URL}
		mockFetcher.EXPECT().
			Fetch(gomock.Any(), link.URL, gomock.Any()).
			Return([]domain.UpdateEvent{mockEvent}, nil).AnyTimes()

		mockLinkRepo.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(link, nil).AnyTimes()
	}

	fetcherService := NewFetcherService([]domain.LinkFetcher{mockFetcher})
	service, err := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, batchSize, 2, newLogger())
	require.NoError(t, err)

	err = service.GetUpdates(context.Background())
	require.NoError(t, err)

	assert.Len(t, notifier.SentUpdates, 3, "Expected 3 updates, one for each link")

	var updatedURLs []string
	for _, u := range notifier.SentUpdates {
		updatedURLs = append(updatedURLs, u.URL)
	}
	assert.Contains(t, updatedURLs, linksBatch1[0].URL)
	assert.Contains(t, updatedURLs, linksBatch1[1].URL)
	assert.Contains(t, updatedURLs, linksBatch2[0].URL)
}

func TestScrapperService_GetUpdates_PartialFailureIsolation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockFetcher := mocks.NewMockLinkFetcher(ctrl)

	notifier := &MockNotifier{}
	batchSize := 10

	goodLink1 := domain.Link{ID: 1, URL: "https://github.com/good1"}
	badLink := domain.Link{ID: 2, URL: "https://github.com/bad"}
	goodLink2 := domain.Link{ID: 3, URL: "https://github.com/good2"}

	mockLinkRepo.EXPECT().GetBatch(gomock.Any(), batchSize, 0).Return([]domain.Link{goodLink1, badLink, goodLink2}, nil)
	mockLinkRepo.EXPECT().GetBatch(gomock.Any(), batchSize, batchSize).Return([]domain.Link{}, nil)

	mockFetcher.EXPECT().CanHandle(gomock.Any()).Return(true).AnyTimes()

	mockSubRepo.EXPECT().GetByLinkID(gomock.Any(), gomock.Any()).
		Return([]domain.Subscription{{ChatID: 100}}, nil).AnyTimes()

	mockFetcher.EXPECT().Fetch(gomock.Any(), goodLink1.URL, gomock.Any()).
		Return([]domain.UpdateEvent{TestUpdateEvent{Desc: "Success 1"}}, nil)
	mockFetcher.EXPECT().Fetch(gomock.Any(), goodLink2.URL, gomock.Any()).
		Return([]domain.UpdateEvent{TestUpdateEvent{Desc: "Success 2"}}, nil)

	mockFetcher.EXPECT().Fetch(gomock.Any(), badLink.URL, gomock.Any()).
		Return(nil, errors.New("external api timeout"))

	mockLinkRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(goodLink1, nil).AnyTimes()
	mockLinkRepo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(goodLink2, nil).AnyTimes()

	fetcherService := NewFetcherService([]domain.LinkFetcher{mockFetcher})
	service, err := NewScrapperService(mockLinkRepo, mockSubRepo, fetcherService, notifier, batchSize, 2, newLogger())
	require.NoError(t, err)

	err = service.GetUpdates(context.Background())

	require.NoError(t, err)

	var descriptions []string
	for _, u := range notifier.SentUpdates {
		descriptions = append(descriptions, u.Description)
	}

	assert.Contains(t, descriptions, "Success 1")
	assert.Contains(t, descriptions, "Success 2")
}
