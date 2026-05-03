package application

import (
	"context"
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

type mockLinkValidator struct {
	Can []bool
}

func (m *mockLinkValidator) CanHandle(url string) bool {
	if len(m.Can) == 0 {
		return false
	}
	can := m.Can[0]
	m.Can = m.Can[1:]
	return can
}

type mockTransactor struct{}

func (m mockTransactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSubscriptionService_AddLink_NewLinkCreatedAndSaved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockChatRepo := mocks.NewMockChatRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockValidator := &mockLinkValidator{Can: []bool{true}}

	service := NewSubscriptionService(mockChatRepo, mockLinkRepo, mockSubRepo, mockTransactor{}, mockValidator, testLogger())

	ctx := context.Background()
	chatID := int64(123)
	url := "https://github.com/user/repo"
	tags := []string{"work", "important"}

	mockChatRepo.EXPECT().GetByID(ctx, chatID).Return(domain.Chat{ID: chatID}, nil)

	mockLinkRepo.EXPECT().GetByURL(ctx, url).Return(domain.Link{}, domain.ErrLinkNotFound)

	expectedSavedLink := domain.Link{
		ID:          1,
		URL:         url,
		LastUpdated: time.Now(),
	}
	mockLinkRepo.EXPECT().Save(ctx, gomock.Any()).Return(expectedSavedLink, nil)

	mockSubRepo.EXPECT().Exists(ctx, chatID, expectedSavedLink.ID).Return(false, nil)

	expectedSub := domain.Subscription{
		ChatID: chatID,
		LinkID: expectedSavedLink.ID,
		Tags:   tags,
	}
	mockSubRepo.EXPECT().Save(ctx, expectedSub).Return(nil)

	trackedLink, err := service.AddLink(ctx, chatID, url, tags)

	require.NoError(t, err)
	assert.Equal(t, expectedSavedLink.ID, trackedLink.ID)
	assert.Equal(t, expectedSavedLink.URL, trackedLink.URL)
	assert.Equal(t, tags, trackedLink.Tags)
}

func TestSubscriptionService_AddLink_ExistingLinkJustSubscribed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockChatRepo := mocks.NewMockChatRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockValidator := &mockLinkValidator{Can: []bool{true}}

	service := NewSubscriptionService(mockChatRepo, mockLinkRepo, mockSubRepo, mockTransactor{}, mockValidator, testLogger())

	ctx := context.Background()
	chatID := int64(123)
	url := "https://github.com/user/repo"
	tags := []string{"go"}

	existingLink := domain.Link{
		ID:          42,
		URL:         url,
		LastUpdated: time.Now().Add(-1 * time.Hour),
	}

	mockChatRepo.EXPECT().GetByID(ctx, chatID).Return(domain.Chat{ID: chatID}, nil)

	mockLinkRepo.EXPECT().GetByURL(ctx, url).Return(existingLink, nil)

	mockSubRepo.EXPECT().Exists(ctx, chatID, existingLink.ID).Return(false, nil)

	expectedSub := domain.Subscription{
		ChatID: chatID,
		LinkID: existingLink.ID,
		Tags:   tags,
	}
	mockSubRepo.EXPECT().Save(ctx, expectedSub).Return(nil)

	trackedLink, err := service.AddLink(ctx, chatID, url, tags)

	require.NoError(t, err)
	assert.Equal(t, existingLink.ID, trackedLink.ID)
	assert.Equal(t, tags, trackedLink.Tags)
}

func TestSubscriptionService_AddLink_UnsupportedURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockChatRepo := mocks.NewMockChatRepository(ctrl)
	mockLinkRepo := mocks.NewMockLinkRepository(ctrl)
	mockSubRepo := mocks.NewMockSubscriptionRepository(ctrl)
	mockValidator := &mockLinkValidator{Can: []bool{false}}

	service := NewSubscriptionService(mockChatRepo, mockLinkRepo, mockSubRepo, mockTransactor{}, mockValidator, testLogger())

	ctx := context.Background()
	chatID := int64(123)
	url := "https://unsupported.com/page"

	mockChatRepo.EXPECT().GetByID(ctx, chatID).Return(domain.Chat{ID: chatID}, nil)

	_, err := service.AddLink(ctx, chatID, url, nil)

	require.ErrorIs(t, err, domain.ErrURLNotSupported)
}
