package stackoverflow

import (
	"fmt"
	"time"
)

type StackoverflowAnswerUpdate struct {
	Title     string
	Owner     string
	CreatedAt time.Time
	Body      string
}

func (soAnswerUpdate *StackoverflowAnswerUpdate) UpdatedAt() time.Time {
	return soAnswerUpdate.CreatedAt
}

func (soAnswerUpdate *StackoverflowAnswerUpdate) Description() string {
	return fmt.Sprintf("New answer on question \"%s\" by %s", soAnswerUpdate.Title, soAnswerUpdate.Owner)
}

func (soAnswerUpdate *StackoverflowAnswerUpdate) Preview() string {
	return soAnswerUpdate.Body
}

type StackoverflowCommentUpdate struct {
	Title     string
	Owner     string
	CreatedAt time.Time
	Body      string
}

func (soCommentUpdate *StackoverflowCommentUpdate) UpdatedAt() time.Time {
	return soCommentUpdate.CreatedAt
}

func (soCommentUpdate *StackoverflowCommentUpdate) Description() string {
	return fmt.Sprintf("New comment on question \"%s\" by %s", soCommentUpdate.Title, soCommentUpdate.Owner)
}

func (soCommentUpdate *StackoverflowCommentUpdate) Preview() string {
	return soCommentUpdate.Body
}
