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
	title     string
	owner     string
	createdAt time.Time
	body      string

	maxPreviewLen int
}

func (soAnswerUpdate *AnswerUpdate) UpdatedAt() time.Time {
	return soAnswerUpdate.createdAt
}

func (soAnswerUpdate *AnswerUpdate) Author() string {
	return soAnswerUpdate.owner
}

func (soAnswerUpdate *AnswerUpdate) Description() string {
	return fmt.Sprintf("New answer on question \"%s\" by %s", soAnswerUpdate.title, soAnswerUpdate.owner)
}

func (soAnswerUpdate *AnswerUpdate) Preview() string {
	return truncateText(cleanText(soAnswerUpdate.body), soAnswerUpdate.maxPreviewLen)
}

type CommentUpdate struct {
	title     string
	owner     string
	createdAt time.Time
	body      string

	maxPreviewLen int
}

func (soCommentUpdate *CommentUpdate) UpdatedAt() time.Time {
	return soCommentUpdate.createdAt
}

func (soCommentUpdate *CommentUpdate) Author() string {
	return soCommentUpdate.owner
}

func (soCommentUpdate *CommentUpdate) Description() string {
	return fmt.Sprintf("New comment on question \"%s\" by %s", soCommentUpdate.title, soCommentUpdate.owner)
}

func (soCommentUpdate *CommentUpdate) Preview() string {
	return truncateText(soCommentUpdate.body, soCommentUpdate.maxPreviewLen)
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)

	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
