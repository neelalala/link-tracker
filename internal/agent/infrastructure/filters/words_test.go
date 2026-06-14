package filters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWords_Check_StopWordPresent(t *testing.T) {
	filter := NewWords([]string{"spam", "banned"})
	update1 := makeUpdate("alice", "this is spam content")
	update2 := makeUpdate("alice", "banned activity detected")

	assert.False(t, filter.Check(update1))
	assert.False(t, filter.Check(update2))
}

func TestWords_Check_NoStopWord(t *testing.T) {
	filter := NewWords([]string{"spam", "banned"})
	update := makeUpdate("alice", "normal update about the project")

	assert.True(t, filter.Check(update))
}

func TestWords_Check_EmptyExcludedList(t *testing.T) {
	filter := NewWords([]string{})
	update := makeUpdate("alice", "any text whatsoever")

	assert.True(t, filter.Check(update))
}

func TestWords_Check_PartialWordNotFiltered(t *testing.T) {
	filter := NewWords([]string{"spam"})
	update := makeUpdate("alice", "spammer activity here")

	assert.True(t, filter.Check(update))
}
