package filters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLength_Check_BelowMinLength(t *testing.T) {
	filter := NewLength(50)
	update := makeUpdate("alice", "short")

	assert.False(t, filter.Check(update))
}

func TestLength_Check_AboveMinLength(t *testing.T) {
	filter := NewLength(5)
	update := makeUpdate("alice", "hello world")

	assert.True(t, filter.Check(update))
}

func TestLength_Check_EmptyDescription(t *testing.T) {
	filter := NewLength(1)
	update := makeUpdate("alice", "")

	assert.False(t, filter.Check(update))
}
