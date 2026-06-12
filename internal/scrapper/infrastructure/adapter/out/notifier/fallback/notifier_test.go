package fallback

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
)

type mockNotifier struct {
	err error
}

func (n mockNotifier) SendUpdate(_ context.Context, _ domain.LinkUpdate) error {
	return n.err
}

func TestFallbackNotifier(t *testing.T) {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var (
		errPrimaryFailed   = errors.New("primary failed")
		errSecondaryFailed = errors.New("secondary failed")
	)

	tests := []struct {
		name      string
		primary   domain.UpdateNotifier
		secondary domain.UpdateNotifier
		err       error
	}{
		{
			name:      "primary failed secondary succeed",
			primary:   mockNotifier{errPrimaryFailed},
			secondary: mockNotifier{nil},
			err:       nil,
		},
		{
			name:      "primary failed secondary failed",
			primary:   mockNotifier{errPrimaryFailed},
			secondary: mockNotifier{errSecondaryFailed},
			err:       errors.Join(errPrimaryFailed, errSecondaryFailed),
		},
		{
			name:      "primary succeed",
			primary:   mockNotifier{nil},
			secondary: mockNotifier{errSecondaryFailed},
			err:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notifier := New(tt.primary, tt.secondary, log)

			ctx := context.Background()
			err := notifier.SendUpdate(ctx, domain.LinkUpdate{})
			assert.Equal(t, tt.err, err)
		})
	}
}
