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

func TestUntrackCommand_Execute(t *testing.T) {
	type mockBehavior func(t *testing.T, r *mocks.MockSessionRepository, chatID int64)

	tests := []struct {
		name           string
		msg            domain.Message
		mockBehavior   mockBehavior
		expectedResult string
		expectedError  bool
	}{
		{
			name: "Success",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(t *testing.T, r *mocks.MockSessionRepository, chatID int64) {
				session := domain.NewSession(chatID)
				r.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(session, nil)

				expectedSession := session
				expectedSession.State = domain.StateWaitingForURLUntrack
				r.EXPECT().Save(gomock.Any(), expectedSession).Return(nil)
			},
			expectedResult: untrackCommandTrackingSuccessfully,
			expectedError:  false,
		},
		{
			name: "Error getting session",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(t *testing.T, r *mocks.MockSessionRepository, chatID int64) {
				r.EXPECT().GetOrCreate(gomock.Any(), chatID).Return(domain.Session{}, errors.New("unexpected error"))
			},
			expectedResult: untrackCommandUnexpectedError,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repoMock := mocks.NewMockSessionRepository(ctrl)
			tt.mockBehavior(t, repoMock, tt.msg.ChatID)

			cmd := NewUntrackCommand(repoMock, discardLogger())

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
