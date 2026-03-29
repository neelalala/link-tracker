package application

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
	"os"
	"testing"
)

type MockScrapper struct {
	GetTrackedLinksFunc func(chatID int64) ([]domain.TrackedLink, error)
	AddLinkFunc         func(chatID int64, url string, tags []string) (domain.TrackedLink, error)
	RemoveLinkFunc      func(chatID int64, url string) (domain.TrackedLink, error)
	RegisterChatFunc    func(chatID int64) error
	DeleteChatFunc      func(chatID int64) error
}

func (scrapper *MockScrapper) GetTrackedLinks(ctx context.Context, chatID int64) ([]domain.TrackedLink, error) {
	if scrapper.GetTrackedLinksFunc != nil {
		return scrapper.GetTrackedLinksFunc(chatID)
	}
	return nil, nil
}

func (scrapper *MockScrapper) AddLink(ctx context.Context, chatID int64, url string, tags []string) (domain.TrackedLink, error) {
	if scrapper.AddLinkFunc != nil {
		return scrapper.AddLinkFunc(chatID, url, tags)
	}
	return domain.TrackedLink{}, nil
}

func (scrapper *MockScrapper) RemoveLink(context.Context, int64, string) (domain.TrackedLink, error) {
	return domain.TrackedLink{}, nil
}

func (scrapper *MockScrapper) RegisterChat(context.Context, int64) error { return nil }

func (scrapper *MockScrapper) DeleteChat(context.Context, int64) error { return nil }

func logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestCommandService_HandleList(t *testing.T) {
	tests := []struct {
		name           string
		message        string
		mockLinks      []domain.TrackedLink
		mockErr        error
		expectedResult string
	}{
		{
			name:           "/list and user has no active subscriptions",
			message:        "/list",
			mockLinks:      []domain.TrackedLink{},
			mockErr:        nil,
			expectedResult: "You have no tracked links.",
		},
		{
			name:    "/list and user has active subscriptions",
			message: "/list",
			mockLinks: []domain.TrackedLink{
				{URL: "https://github.com/user/repo1", Tags: []string{"work"}},
				{URL: "https://stackoverflow.com/questions/123", Tags: []string{}},
			},
			mockErr:        nil,
			expectedResult: "Your tracked links:\nhttps://github.com/user/repo1\n  Tags: work\n\nhttps://stackoverflow.com/questions/123",
		},
		{
			name:           "Scrapper returns an error",
			message:        "/list",
			mockLinks:      nil,
			mockErr:        errors.New("some error"),
			expectedResult: "Something went wrong while getting your links.",
		},
		{
			name:    "User requests /list <tag> and has matching subscriptions",
			message: "/list work",
			mockLinks: []domain.TrackedLink{
				{URL: "https://github.com/user/repo1", Tags: []string{"work", "go"}},
				{URL: "https://stackoverflow.com/questions/123", Tags: []string{"hobby"}},
			},
			mockErr:        nil,
			expectedResult: "Your tracked links with tags work:\nhttps://github.com/user/repo1\n  Tags: work, go",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockScrapper := &MockScrapper{
				GetTrackedLinksFunc: func(chatID int64) ([]domain.TrackedLink, error) {
					return tt.mockLinks, tt.mockErr
				},
			}

			service := NewCommandService(mockScrapper, logger())

			result := service.HandleMessage(ctx, 123, tt.message)

			assert.Equalf(t, tt.expectedResult, result, "expected %q, got %q", tt.expectedResult, result)
		})
	}
}

func TestCommandService_TrackFlow(t *testing.T) {
	type dialogStep struct {
		userMessage    string
		expectedBotMsg string
	}

	tests := []struct {
		name        string
		mockAddLink func(chatID int64, url string, tags []string) (domain.TrackedLink, error)
		dialog      []dialogStep
	}{
		{
			name: "Positive: user successfully tracks a link",
			mockAddLink: func(chatID int64, url string, tags []string) (domain.TrackedLink, error) {
				return domain.TrackedLink{URL: url, Tags: tags}, nil
			},
			dialog: []dialogStep{
				{
					userMessage:    "/track",
					expectedBotMsg: "Please send the link you want to track. Send /cancel to abort.",
				},
				{
					userMessage:    "https://github.com/user/repo",
					expectedBotMsg: "Link saved! Now send tags separated by commas (e.g., work, bug). Or send 'skip' to add without tags.",
				},
				{
					userMessage:    "skip",
					expectedBotMsg: "Success! Now tracking link: https://github.com/user/repo",
				},
			},
		},
		{
			name: "Negative: user tries to track already tracked link",
			mockAddLink: func(chatID int64, url string, tags []string) (domain.TrackedLink, error) {
				return domain.TrackedLink{}, domain.ErrAlreadySubscribed
			},
			dialog: []dialogStep{
				{
					userMessage:    "/track",
					expectedBotMsg: "Please send the link you want to track. Send /cancel to abort.",
				},
				{
					userMessage:    "https://github.com/user/repo",
					expectedBotMsg: "Link saved! Now send tags separated by commas (e.g., work, bug). Or send 'skip' to add without tags.",
				},
				{
					userMessage:    "skip",
					expectedBotMsg: "You're already tracking this link.",
				},
			},
		},
		{
			name: "Negative: user sends invalid/unsupported link",
			mockAddLink: func(chatID int64, url string, tags []string) (domain.TrackedLink, error) {
				return domain.TrackedLink{}, domain.ErrUrlNotSupported
			},
			dialog: []dialogStep{
				{
					userMessage:    "/track",
					expectedBotMsg: "Please send the link you want to track. Send /cancel to abort.",
				},
				{
					userMessage:    "tbank://github.com/user/repo",
					expectedBotMsg: "Link saved! Now send tags separated by commas (e.g., work, bug). Or send 'skip' to add without tags.",
				},
				{
					userMessage:    "skip",
					expectedBotMsg: "This link is not supported yet.",
				},
			},
		},
		{
			name:        "User cancels the process",
			mockAddLink: nil,
			dialog: []dialogStep{
				{
					userMessage:    "/track",
					expectedBotMsg: "Please send the link you want to track. Send /cancel to abort.",
				},
				{
					userMessage:    "/cancel",
					expectedBotMsg: "Tracking process cancelled.",
				},
			},
		},
		{
			name:        "User cancels the process with another command",
			mockAddLink: nil,
			dialog: []dialogStep{
				{
					userMessage:    "/track",
					expectedBotMsg: "Please send the link you want to track. Send /cancel to abort.",
				},
				{
					userMessage:    "/list",
					expectedBotMsg: "Tracking process cancelled.\n\nYou have no tracked links.",
				},
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockScrapper := &MockScrapper{
				AddLinkFunc: tt.mockAddLink,
			}
			service := NewCommandService(mockScrapper, logger())
			chatID := int64(47)

			for i, step := range tt.dialog {
				result := service.HandleMessage(ctx, chatID, step.userMessage)
				assert.Equalf(t, step.expectedBotMsg, result, "Step %d: sent %q\nexpected: %q\ngot: %q", i+1, step.userMessage, step.expectedBotMsg, result)
			}
		})
	}
}
