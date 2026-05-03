package commands

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

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCancelCommand_Execute(t *testing.T) {
	type mockBehavior func(r *mocks.MockSessionRepository, chatID int64)

	tests := []struct {
		name           string
		msg            domain.Message
		mockBehavior   mockBehavior
		expectedResult string
		expectedError  bool
	}{
		{
			name: "Success (Process canceled)",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(r *mocks.MockSessionRepository, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateWaitingForURLTrack}
				r.EXPECT().Delete(gomock.Any(), chatID).Return(session, nil)
			},
			expectedResult: cancelCommandCanceled,
			expectedError:  false,
		},
		{
			name: "Success (Nothing to cancel)",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(r *mocks.MockSessionRepository, chatID int64) {
				session := domain.Session{ChatID: chatID, State: domain.StateIdle}
				r.EXPECT().Delete(gomock.Any(), chatID).Return(session, nil)
			},
			expectedResult: cancelCommandNothingToCancel,
			expectedError:  false,
		},
		{
			name: "Error deleting session",
			msg:  domain.Message{ChatID: 123},
			mockBehavior: func(r *mocks.MockSessionRepository, chatID int64) {
				r.EXPECT().Delete(gomock.Any(), chatID).Return(domain.Session{}, errors.New("unexpected error"))
			},
			expectedResult: cancelCommandSessionDeleteError,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repoMock := mocks.NewMockSessionRepository(ctrl)
			tt.mockBehavior(repoMock, tt.msg.ChatID)

			cmd := NewCancelCommand(repoMock, discardLogger())

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
