package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/mocks"
	"go.uber.org/mock/gomock"
)

var testUnexpectedError = errors.New("unexpected error")

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestDialogService_HandleMessage(t *testing.T) {
	type mockBehavior func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64)

	tests := []struct {
		name           string
		msg            domain.Message
		mockBehavior   mockBehavior
		expectedResult string
		expectedError  error
	}{
		{
			name: "Error getting session",
			msg:  domain.Message{ChatID: 123, Text: "hello"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(domain.Session{}, testUnexpectedError)
			},
			expectedResult: dialogServiceErrorGettingSession,
			expectedError:  testUnexpectedError,
		},
		{
			name: "Unknown state fallback",
			msg:  domain.Message{ChatID: 123, Text: "hello"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				invalidSession := domain.Session{ChatID: chatID, State: domain.SessionState("broken_state")}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(invalidSession, nil)

				resetSession := domain.Session{ChatID: chatID, State: domain.StateIdle}
				repo.EXPECT().Save(gomock.Any(), resetSession).Return(nil)
			},
			expectedResult: dialogServiceErrorUnknownState,
			expectedError:  domain.ErrBadSessionState,
		},

		{
			name: "State Idle text",
			msg:  domain.Message{ChatID: 123, Text: "just chatting"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateIdle}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)
			},
			expectedResult: dialogServiceIdleState,
			expectedError:  nil,
		},

		{
			name: "WaitingForURLTrack Empty URL",
			msg:  domain.Message{ChatID: 123, Text: "   \n "},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForURLTrack}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)
			},
			expectedResult: dialogServiceErrorEmptyURL,
			expectedError:  nil,
		},
		{
			name: "WaitingForURLTrack Success",
			msg:  domain.Message{ChatID: 123, Text: "https://github.com"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForURLTrack}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)

				expectedSession := domain.Session{ChatID: chatID, State: domain.StateWaitingForTags, URL: "https://github.com"}
				repo.EXPECT().Save(gomock.Any(), expectedSession).Return(nil)
			},
			expectedResult: dialogServiceTrackLinkSaved,
			expectedError:  nil,
		},

		{
			name: "WaitingForTags Skip tags (Success)",
			msg:  domain.Message{ChatID: 123, Text: "sKiP"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForTags, URL: "https://test.com"}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)

				scrapper.EXPECT().AddLink(gomock.Any(), chatID, "https://test.com", gomock.Nil()).Return(domain.TrackedLink{}, nil)

				resetSession := domain.Session{ChatID: chatID, State: domain.StateIdle}
				repo.EXPECT().Save(gomock.Any(), resetSession).Return(nil)
			},
			expectedResult: "Success! Now tracking link: https://test.com",
			expectedError:  nil,
		},
		{
			name: "WaitingForTags With tags (Success)",
			msg:  domain.Message{ChatID: 123, Text: " go, backend ,  news  "},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForTags, URL: "https://test.com"}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)

				expectedTags := []string{"go", "backend", "news"}
				scrapper.EXPECT().AddLink(gomock.Any(), chatID, "https://test.com", expectedTags).Return(domain.TrackedLink{}, nil)

				resetSession := domain.Session{ChatID: chatID, State: domain.StateIdle}
				repo.EXPECT().Save(gomock.Any(), resetSession).Return(nil)
			},
			expectedResult: "Success! Now tracking link: https://test.com",
			expectedError:  nil,
		},
		{
			name: "WaitingForTags Already tracking error",
			msg:  domain.Message{ChatID: 123, Text: "skip"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForTags, URL: "https://test.com"}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)

				scrapper.EXPECT().AddLink(gomock.Any(), chatID, "https://test.com", gomock.Any()).Return(domain.TrackedLink{}, domain.ErrAlreadySubscribed)
			},
			expectedResult: "You are already tracking this link",
			expectedError:  nil,
		},

		{
			name: "WaitingForURLUntrack Success",
			msg:  domain.Message{ChatID: 123, Text: "https://test.com"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForURLUntrack, URL: ""}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)

				scrapper.EXPECT().RemoveLink(gomock.Any(), chatID, "https://test.com").Return(domain.TrackedLink{}, nil)

				resetSession := domain.Session{ChatID: chatID, State: domain.StateIdle}
				repo.EXPECT().Save(gomock.Any(), resetSession).Return(nil)
			},
			expectedResult: "Link https://test.com has been untracked",
			expectedError:  nil,
		},
		{
			name: "WaitingForURLUntrack Not subscribed error",
			msg:  domain.Message{ChatID: 123, Text: "https://test.com"},
			mockBehavior: func(repo *mocks.MockSessionRepository, scrapper *mocks.MockScrapper, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForURLUntrack}
				repo.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)

				scrapper.EXPECT().RemoveLink(gomock.Any(), chatID, "https://test.com").Return(domain.TrackedLink{}, domain.ErrNotSubscribed)
			},
			expectedResult: "You are not tracking this link",
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repoMock := mocks.NewMockSessionRepository(ctrl)
			scrapperMock := mocks.NewMockScrapper(ctrl)

			tt.mockBehavior(repoMock, scrapperMock, tt.msg.ChatID)

			service := NewDialogService(scrapperMock, repoMock, discardLogger())

			res, err := service.HandleMessage(context.Background(), tt.msg)

			assert.Equal(t, tt.expectedResult, res)
			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
