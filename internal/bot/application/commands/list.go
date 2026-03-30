package commands

import (
	"context"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
	"slices"
	"strings"
)

const (
	listCommandName        = "list"
	listCommandDescription = "List your tracked links. You can filter them by specifying tags"

	listCommandUnexpectedError = "Something went wrong while getting your links. Try again"
	listCommandNoTrackedLinks  = "You have no tracked links"
)

type ListCommand struct {
	scrapper domain.Scrapper
	logger   *slog.Logger
}

func NewListCommand(scrapper domain.Scrapper, logger *slog.Logger) *ListCommand {
	return &ListCommand{
		scrapper: scrapper,
		logger:   logger,
	}
}

func (c *ListCommand) Name() string {
	return listCommandName
}

func (c *ListCommand) Description() string {
	return listCommandDescription
}

func (c *ListCommand) Execute(ctx context.Context, msg domain.Message) (string, error) {
	links, err := c.scrapper.GetTrackedLinks(ctx, msg.ChatID)
	if err != nil {
		c.logger.Error("error getting tracked links from scrapper",
			slog.String("error", err.Error()),
			slog.Int64("chat_id", msg.ChatID),
		)
		return listCommandUnexpectedError, err
	}

	if len(links) == 0 {
		return listCommandNoTrackedLinks, nil
	}

	_, args := msg.ParseCommand()

	sb := strings.Builder{}

	if len(args) > 0 {
		tags := make([]string, 0, len(args))
		for _, arg := range args {
			tags = append(tags, strings.TrimSpace(strings.TrimSuffix(arg, ",")))
		}

		links = filterWithTags(links, tags)
		if len(links) == 0 {
			sb.WriteString("You have no tracked links with tags ")
			for _, arg := range args {
				sb.WriteString(arg)
			}
			return sb.String(), nil
		}
	}

	sb.WriteString("Your tracked links")
	if len(args) > 0 {
		sb.WriteString(" with tags ")
		for i, arg := range args {
			sb.WriteString(arg)
			if i < len(args)-1 {
				sb.WriteString(" ")
			}
		}
	}

	sb.WriteString(":\n")
	for i, link := range links {
		sb.WriteString(link.URL)
		if len(link.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("\n  Tags: %s", strings.Join(link.Tags, ", ")))
		}
		if i != len(links)-1 {
			sb.WriteString("\n\n")
		}
	}

	return sb.String(), nil
}

func filterWithTags(links []domain.TrackedLink, tags []string) []domain.TrackedLink {
	if len(tags) == 0 {
		return links
	}

	var filteredLinks []domain.TrackedLink

	for _, link := range links {
		hasAllTags := true
		for _, requiredTag := range tags {
			if !slices.Contains(link.Tags, requiredTag) {
				hasAllTags = false
				break
			}
		}

		if hasAllTags {
			filteredLinks = append(filteredLinks, link)
		}
	}

	return filteredLinks
}
