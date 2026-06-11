package stackoverflow

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

var stripHtmlRegex = regexp.MustCompile(`<[^>]*>`)

func cleanText(text string) string {
	clean := stripHtmlRegex.ReplaceAllString(text, "")

	clean = html.UnescapeString(clean)

	return strings.TrimSpace(clean)
}

type AnswerUpdate struct {
	Title     string
	Owner     string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (soAnswerUpdate *AnswerUpdate) UpdatedAt() time.Time {
	return soAnswerUpdate.CreatedAt
}

func (soAnswerUpdate *AnswerUpdate) Description() string {
	return fmt.Sprintf("New answer on question \"%s\" by %s", soAnswerUpdate.Title, soAnswerUpdate.Owner)
}

func (soAnswerUpdate *AnswerUpdate) Preview() string {
	return truncateText(cleanText(soAnswerUpdate.Body), soAnswerUpdate.MaxPreviewLen)
}

type CommentUpdate struct {
	Title     string
	Owner     string
	CreatedAt time.Time
	Body      string

	MaxPreviewLen int
}

func (soCommentUpdate *CommentUpdate) UpdatedAt() time.Time {
	return soCommentUpdate.CreatedAt
}

func (soCommentUpdate *CommentUpdate) Description() string {
	return fmt.Sprintf("New comment on question \"%s\" by %s", soCommentUpdate.Title, soCommentUpdate.Owner)
}

func (soCommentUpdate *CommentUpdate) Preview() string {
	return truncateText(soCommentUpdate.Body, soCommentUpdate.MaxPreviewLen)
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
