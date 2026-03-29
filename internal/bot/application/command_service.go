package application

import (
	"context"
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	"log/slog"
	"strings"
	"sync"
)

type Scrapper interface {
	RegisterChat(ctx context.Context, chatId int64) error
	DeleteChat(ctx context.Context, chatId int64) error
	GetTrackedLinks(ctx context.Context, chatId int64) ([]domain.TrackedLink, error)
	AddLink(ctx context.Context, chatId int64, url string, tags []string) (domain.TrackedLink, error)
	RemoveLink(ctx context.Context, chatId int64, url string) (domain.TrackedLink, error)
}

type CommandService struct {
	scrapper Scrapper
	logger   *slog.Logger

	mu       sync.RWMutex
	sessions map[int64]*domain.TrackSession
}

func NewCommandService(scrapper Scrapper, logger *slog.Logger) *CommandService {
	return &CommandService{
		scrapper: scrapper,
		logger:   logger,
		sessions: make(map[int64]*domain.TrackSession),
	}
}

func (service *CommandService) HandleMessage(ctx context.Context, chatID int64, text string) string {
	text = strings.TrimSpace(text)

	sb := strings.Builder{}
	service.mu.RLock()
	session, exists := service.sessions[chatID]
	service.mu.RUnlock()

	if exists && strings.HasPrefix(text, "/") {
		service.clearSession(chatID)
		sb.WriteString("Tracking process cancelled.\n\n")
		exists = false
	}

	// TODO can start /track even if not registered
	if exists && session.State != domain.StateIdle {
		sb.WriteString(service.processSM(ctx, chatID, text, session))
		return sb.String()
	}

	parts := strings.Fields(text)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "/") {
		sb.WriteString("I only understand commands. Try /help.")
		return sb.String()
	}

	commandName := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	sb.WriteString(service.executeCommand(ctx, chatID, commandName, args))
	return strings.TrimSpace(sb.String())
}

func (service *CommandService) clearSession(chatID int64) {
	service.mu.Lock()
	defer service.mu.Unlock()
	delete(service.sessions, chatID)
}

func (service *CommandService) processSM(ctx context.Context, chatID int64, text string, session *domain.TrackSession) string {
	service.mu.Lock()
	defer service.mu.Unlock()

	switch session.State {

	case domain.StateWaitingForURL:
		session.URL = text
		session.State = domain.StateWaitingForTags
		return "Link saved! Now send tags separated by commas (e.g., work, bug). Or send 'skip' to add without tags."

	case domain.StateWaitingForTags:
		var tags []string
		if strings.ToLower(text) != "skip" {
			rawTags := strings.Split(text, ",")
			for _, t := range rawTags {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		_, err := service.scrapper.AddLink(ctx, chatID, session.URL, tags)

		delete(service.sessions, chatID)

		if err != nil {
			if errors.Is(err, domain.ErrAlreadySubscribed) {
				return "You're already tracking this link."
			}
			if errors.Is(err, domain.ErrChatNotRegistered) {
				return "Please register before tracking any link. Just use /start :)"
			}
			if errors.Is(err, domain.ErrUrlNotSupported) {
				return "This link is not supported yet."
			}
			service.logger.Error("Scrapper AddLink failed", slog.String("error", err.Error()))
			return "Something went wrong while saving the link in the scrapper."
		}

		return fmt.Sprintf("Success! Now tracking link: %s", session.URL)
	default:
		return "Unknown state. Process cancelled."
	}
}

func (service *CommandService) executeCommand(ctx context.Context, chatID int64, cmd string, args []string) string {
	switch cmd {
	case "start":
		return service.handleStart(ctx, chatID)
	case "help":
		return service.handleHelp()
	case "track":
		service.mu.Lock()
		service.sessions[chatID] = &domain.TrackSession{State: domain.StateWaitingForURL}
		service.mu.Unlock()
		return "Please send the link you want to track. Send /cancel to abort."
	case "untrack":
		return service.handleUntrack(ctx, chatID, args)
	case "list":
		return service.handleList(ctx, chatID, args)
	case "cancel":
		return ""
	default:
		return "Unknown command. Try /help."
	}
}

func (service *CommandService) handleUntrack(ctx context.Context, chatID int64, args []string) string {
	if len(args) == 0 {
		return "Please provide a link to untrack. Usage: /untrack <link>"
	}
	url := args[0]

	_, err := service.scrapper.RemoveLink(ctx, chatID, url)
	if err != nil {
		if errors.Is(err, domain.ErrLinkNotFound) || errors.Is(err, domain.ErrNotSubscribed) {
			return "You're not tracking this link."
		}
		service.logger.Error("Scrapper RemoveLink failed", slog.String("error", err.Error()))
		return "Something went wrong while untracking the link."
	}

	return fmt.Sprintf("Link %s has been untracked.", url)
}

func (service *CommandService) handleList(ctx context.Context, chatID int64, args []string) string {
	links, err := service.scrapper.GetTrackedLinks(ctx, chatID)
	if err != nil {
		service.logger.Error("Scrapper GetTrackedLinks failed", slog.String("error", err.Error()))
		return "Something went wrong while getting your links."
	}

	if len(links) == 0 {
		return "You have no tracked links."
	}

	if len(args) > 0 {
		tags := make([]string, 0, len(links))
		for _, arg := range args {
			tags = append(tags, strings.TrimSuffix(arg, ","))
		}

		links = service.filterWithTags(links, tags)
		if len(links) == 0 {
			sb := strings.Builder{}
			sb.WriteString("You have no tracked links with tags ")
			for _, arg := range args {
				sb.WriteString(arg)
			}
			return sb.String()
		}
	}

	sb := strings.Builder{}
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

	return sb.String()
}

func (service *CommandService) filterWithTags(links []domain.TrackedLink, tags []string) []domain.TrackedLink {
	filteredLinks := make([]domain.TrackedLink, 0)
Outer:
	for _, link := range links {
		for _, tag := range tags {
			for _, userTag := range link.Tags {
				if userTag == tag {
					filteredLinks = append(filteredLinks, link)
					continue Outer
				}
			}
		}
	}
	return filteredLinks
}

func (service *CommandService) handleStart(ctx context.Context, chatID int64) string {
	err := service.scrapper.RegisterChat(ctx, chatID)
	if err != nil {
		if errors.Is(err, domain.ErrChatAlreadyRegistered) {
			return "Hi again! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
		}
		return "Something went wrong while registering you."
	}
	return "Hi! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
}

func (service *CommandService) handleHelp() string {
	return `Available commands:
/start – what this bot can do
/help – list all available commands
/track – track your links
/untrack ling – stop tracking your link
/list – list all your tracked links`
}

func (service *CommandService) GetCommands() []domain.Command {
	return []domain.Command{
		{
			Name:        "start",
			Description: "What this bot can do",
		},
		{
			Name:        "help",
			Description: "List all available commands",
		},
		{
			Name:        "track",
			Description: "Track your link",
		},
		{
			Name:        "untrack",
			Description: "Stop tracking your link",
		},
		{
			Name:        "list",
			Description: "List all your tracked links",
		},
		{
			Name:        "cancel",
			Description: "Cancel adding link process",
		},
	}
}
