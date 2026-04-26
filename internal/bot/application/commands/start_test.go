package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/mocks"
	"go.uber.org/mock/gomock"
)

func TestStartCommand_Execute(t *testing.T) {
	type mockBehavior func(s *mocks.MockScrapper, chatID int64)

	tests := []struct {
		name           string
		msg            domain.Message
		mockBehavior   mockBehavior
		expectedResult string
		expectedError  bool
	}{
		{
			name: "Success (New User)",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				s.EXPECT().RegisterChat(gomock.Any(), chatID).Return(nil)
			},
			expectedResult: startCommandMessageNewUser,
			expectedError:  false,
		},
		{
			name: "Success (Old User)",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				s.EXPECT().RegisterChat(gomock.Any(), chatID).Return(domain.ErrChatAlreadyRegistered)
			},
			expectedResult: startCommandMessageOldUser,
			expectedError:  false,
		},
		{
			name: "Unexpected Error",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(s *mocks.MockScrapper, chatID int64) {
				s.EXPECT().RegisterChat(gomock.Any(), chatID).Return(errors.New("db down"))
			},
			expectedResult: startCommandUnexpectedError,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			scrapperMock := mocks.NewMockScrapper(ctrl)
			tt.mockBehavior(scrapperMock, tt.msg.ChatID)

			cmd := NewStartCommand(scrapperMock, discardLogger())

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
