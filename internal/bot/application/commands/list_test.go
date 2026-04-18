package commands

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/mocks"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestListCommand_Execute(t *testing.T) {
	type mockBehavior func(s *mocks.MockScrapper, chatID int64)

	tests := []struct {
		name           string
		msg            domain.Message
		mockBehavior   mockBehavior
		expectedResult string
		expectedError  bool
	}{
		{
			name: "Not registered",
			msg:  domain.Message{ChatID: 123, Text: "/list"},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				s.EXPECT().GetTrackedLinks(gomock.Any(), chatID).Return(nil, domain.ErrChatNotRegistered)
			},
			expectedResult: listCommandUserNotRegistered,
			expectedError:  false,
		},
		{
			name: "No tracked links",
			msg:  domain.Message{ChatID: 123, Text: "/list"},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				s.EXPECT().GetTrackedLinks(gomock.Any(), chatID).Return([]domain.TrackedLink{}, nil)
			},
			expectedResult: listCommandNoTrackedLinks,
			expectedError:  false,
		},
		{
			name: "Has links, no tags in query",
			msg:  domain.Message{ChatID: 123, Text: "/list"},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				links := []domain.TrackedLink{
					{URL: "https://github.com", Tags: []string{"dev"}},
					{URL: "https://habr.com", Tags: nil},
				}
				s.EXPECT().GetTrackedLinks(gomock.Any(), chatID).Return(links, nil)
			},
			expectedResult: "Your tracked links:\nhttps://github.com\n  Tags: dev\n\nhttps://habr.com",
			expectedError:  false,
		},
		{
			name: "Has links, filtering by tags match",
			msg:  domain.Message{ChatID: 123, Text: "/list go news"},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				links := []domain.TrackedLink{
					{URL: "https://go.dev", Tags: []string{"go", "news", "backend"}},
					{URL: "https://bad.dev", Tags: []string{"go"}},
				}
				s.EXPECT().GetTrackedLinks(gomock.Any(), chatID).Return(links, nil)
			},
			expectedResult: "Your tracked links with tags go news:\nhttps://go.dev\n  Tags: go, news, backend",
			expectedError:  false,
		},
		{
			name: "Has links, filtering by tags no match",
			msg:  domain.Message{ChatID: 123, Text: "/list unknown"},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				links := []domain.TrackedLink{
					{URL: "https://go.dev", Tags: []string{"go"}},
				}
				s.EXPECT().GetTrackedLinks(gomock.Any(), chatID).Return(links, nil)
			},
			expectedResult: listCommandNoTrackedLinks + " with tags unknown",
			expectedError:  false,
		},
		{
			name: "Unexpected error from scrapper",
			msg:  domain.Message{ChatID: 123, Text: "/list"},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				s.EXPECT().GetTrackedLinks(gomock.Any(), chatID).Return(nil, errors.New("timeout"))
			},
			expectedResult: listCommandUnexpectedError,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			scrapperMock := mocks.NewMockScrapper(ctrl)
			tt.mockBehavior(scrapperMock, tt.msg.ChatID)

			cmd := NewListCommand(scrapperMock, discardLogger())

			res, err := cmd.Execute(context.Background(), tt.msg)

			assert.Equal(t, tt.expectedResult, res)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
