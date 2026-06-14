package transformers

import (
	"context"
	"fmt"

	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/agent/domain"
)

type LLM interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
}

type Summarizer struct {
	llm       LLM
	threshold int
}

func NewSummarizer(llm LLM, threshold int) Summarizer {
	return Summarizer{
		llm:       llm,
		threshold: threshold,
	}
}

func (transformer Summarizer) Transform(ctx context.Context, raw domain.LinkUpdate) (domain.ProcessedLinkUpdate, error) {
	processed := domain.ProcessedLinkUpdate{
		URL:         raw.URL,
		Author:      raw.Author,
		Description: raw.Description,
		Priority:    domain.PriorityHigh,
		TgChatIDs:   raw.TgChatIDs,
	}

	if len(processed.Description) > transformer.threshold {
		prompt := fmt.Sprintf(`Сократи следующее описание до 1-2 предложений (%d символов): 
%s
Твой ответ должен состоять ТОЛЬКО из сокращенного описания, без твоих комментариев`,
			transformer.threshold,
			processed.Description,
		)
		summary, err := transformer.llm.GenerateText(ctx, prompt)
		if err != nil {
			return domain.ProcessedLinkUpdate{}, fmt.Errorf("error generating text: %w", err)
		}

		processed.Description = summary
	}

	return processed, nil
}
