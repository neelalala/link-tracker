package application

import (
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/domain"
	"log/slog"
	"strings"
)

type CommandService struct {
	userRepo domain.UserRepository
	linkRepo domain.LinkRepository
	logger   *slog.Logger
}

func NewCommandService(userRepo domain.UserRepository, linkRepo domain.LinkRepository, logger *slog.Logger) *CommandService {
	return &CommandService{
		userRepo: userRepo,
		linkRepo: linkRepo,
		logger:   logger,
	}
}

func (service *CommandService) GetCommands() []domain.Command {
	return []domain.Command{
		{
			Name:        "start",
			Description: "What this bot can do",
			Do:          service.handleStart,
		},
		{
			Name:        "help",
			Description: "List all available commands",
			Do:          service.handleHelp,
		},
		{
			Name:        "track",
			Description: "Track your link",
			Do:          service.handleTrack,
		},
		{
			Name:        "untrack",
			Description: "Stop tracking your link",
			Do:          service.handleUntrack,
		},
		{
			Name:        "list",
			Description: "List all your tracked links",
			Do:          service.handleList,
		},
	}
}

func (service *CommandService) handleTrack(user domain.User, chatID int64, args []string) string {
	if len(args) == 0 {
		return "Please provide a link to track."
	}

	url := args[0]

	var tags []string
	if len(args) > 1 {
		tags = args[1:]
	}

	link := domain.Link{
		UserID: user.ID,
		ChatID: chatID,
		URL:    url,
		Tags:   tags,
	}

	err := service.linkRepo.Save(link)
	if err != nil {
		service.logger.Error("Cannot save link.", slog.String("context", "linkRepo.Save"), slog.String("link", link.URL), slog.String("error", err.Error()))
		return "Something went wrong while saving the link."
	}

	return fmt.Sprintf("Tracking link %s", link.URL)
}

func (service *CommandService) handleUntrack(user domain.User, chatID int64, args []string) string {
	if len(args) == 0 {
		return "Please provide a link to untrack."
	}

	links, err := service.linkRepo.GetByUserIdChatId(user.ID, chatID)
	if err != nil {
		service.logger.Error("Cannot get link.", slog.String("context", "linkRepo.GetByUserIdChatId"), slog.Int64("userID", user.ID), slog.Int64("chatID", chatID), slog.String("error", err.Error()))
		return "Something went wrong while getting link."
	}

	if len(links) == 0 {
		return service.noLinksTracked()
	}

	url := args[0]
	for _, link := range links {
		if link.URL != url {
			continue
		}
		err := service.linkRepo.Delete(link)
		if err != nil {
			if !errors.Is(err, domain.ErrLinkNotFound) {
				service.logger.Error("Cannot delete link.", slog.String("context", "linkRepo.Delete"), slog.String("error", err.Error()), slog.String("link", link.URL))
				return "Something went wrong while deleting link."
			}
			return fmt.Sprintf("You don't track %s.", link.URL)
		}
		return fmt.Sprintf("Link %s has been untracked.", link.URL)
	}

	return "You're not tracking this link."
}

func (service *CommandService) handleList(user domain.User, chatID int64, args []string) string {
	links, err := service.linkRepo.GetByUserIdChatId(user.ID, chatID)
	if err != nil {
		service.logger.Error("Cannot get link.", slog.String("context", "linkRepo.GetByUserIdChatId"), slog.Int64("userID", user.ID), slog.Int64("chatID", chatID), slog.String("error", err.Error()))
		return "Something went wrong while getting your links."
	}

	if len(links) == 0 {
		return service.noLinksTracked()
	}

	sb := strings.Builder{}
	sb.WriteString("Your tracked links:\n")
	for i, link := range links {
		sb.WriteString(link.URL)
		if link.Tags != nil {
			sb.WriteString(", Tags: ")
			sb.WriteString(strings.Join(link.Tags, ", "))
		}
		if i != len(links)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (service *CommandService) noLinksTracked() string {
	return "You have no tracked links."
}

func (service *CommandService) handleStart(user domain.User, chatID int64, args []string) string {
	return "Hi! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
}

func (service *CommandService) handleHelp(user domain.User, chatID int64, args []string) string {
	return `Available commands:
/start – what this bot can do
/help – list all available commands
/track link [tags] – track your links, you can set tags
/untrack ling – stop tracking your link
/list – list all your tracked links`
}
