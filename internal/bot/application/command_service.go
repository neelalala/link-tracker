package application

import (
	"errors"
	"fmt"
	"gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/bot/domain"
	scrapperdomain "gitlab.education.tbank.ru/backend-academy-go-2025/homeworks/link-tracker/internal/scrapper/domain"
	"log/slog"
	"strings"
	"sync"
)

type TrackState int

const (
	StateIdle TrackState = iota
	StateWaitingForURL
	StateWaitingForTags
	StateWaitingForFilters
)

type TrackSession struct {
	State TrackState
	URL   string
	Tags  []string
}

type Scrapper interface {
	RegisterChat(chatId int64) error
	DeleteChat(chatId int64) error
	GetTrackedLinks(chatId int64) ([]scrapperdomain.TrackedLink, error)
	AddLink(chatId int64, url string, tags, filters []string) (scrapperdomain.TrackedLink, error)
	RemoveLink(chatId int64, url string) (scrapperdomain.TrackedLink, error)
}

type CommandService struct {
	scrapper Scrapper
	logger   *slog.Logger

	mu       sync.RWMutex
	sessions map[int64]*TrackSession
}

func NewCommandService(scrapper Scrapper, logger *slog.Logger) *CommandService {
	return &CommandService{
		scrapper: scrapper,
		logger:   logger,
		sessions: make(map[int64]*TrackSession),
	}
}

func (service *CommandService) HandleMessage(chatID int64, text string) string {
	text = strings.TrimSpace(text)

	if text == "/cancel" {
		service.clearSession(chatID)
		return "Tracking process cancelled."
	}

	service.mu.RLock()
	session, exists := service.sessions[chatID]
	service.mu.RUnlock()

	if exists && strings.HasPrefix(text, "/") {
		service.clearSession(chatID)
		exists = false
	}

	if exists && session.State != StateIdle {
		return service.processSM(chatID, text, session)
	}

	parts := strings.Fields(text)
	if len(parts) == 0 || !strings.HasPrefix(parts[0], "/") {
		return "I only understand commands. Try /help."
	}

	commandName := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	return service.executeCommand(chatID, commandName, args)
}

func (service *CommandService) clearSession(chatID int64) {
	service.mu.Lock()
	defer service.mu.Unlock()
	delete(service.sessions, chatID)
}

func (service *CommandService) processSM(chatID int64, text string, session *TrackSession) string {
	service.mu.Lock()
	defer service.mu.Unlock()

	switch session.State {

	case StateWaitingForURL:
		session.URL = text
		session.State = StateWaitingForTags
		return "Link saved! Now send tags separated by commas (e.g., work, bug). Or send 'skip' to add without tags."

	case StateWaitingForTags:
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

		session.Tags = tags
		session.State = StateWaitingForFilters
		return "Tags saved! Now send filters separated by commas (e.g., work, bug). Or send 'skip' to add without filters."
	case StateWaitingForFilters:
		var filters []string

		_, err := service.scrapper.AddLink(chatID, session.URL, session.Tags, filters)

		delete(service.sessions, chatID)

		if err != nil {
			if errors.Is(err, scrapperdomain.ErrAlreadySubscribed) {
				return "You're already tracking this link."
			}
			service.logger.Error("Scrapper AddLink failed", slog.String("error", err.Error()))
			return "Something went wrong while saving the link in the scrapper."
		}

		return fmt.Sprintf("Success! Now tracking link: %s", session.URL)
	}

	return "Unknown state. Process cancelled."
}

func (service *CommandService) executeCommand(chatID int64, cmd string, args []string) string {
	switch cmd {
	case "start":
		return service.handleStart(chatID, args)
	case "help":
		return service.handleHelp(chatID, args)
	case "track":
		service.mu.Lock()
		service.sessions[chatID] = &TrackSession{State: StateWaitingForURL}
		service.mu.Unlock()
		return "Please send the link you want to track. Send /cancel to abort."
	case "untrack":
		return service.handleUntrack(chatID, args)
	case "list":
		return service.handleList(chatID, args)
	default:
		return "Unknown command. Try /help."
	}
}

func (service *CommandService) handleUntrack(chatID int64, args []string) string {
	if len(args) == 0 {
		return "Please provide a link to untrack. Usage: /untrack <link>"
	}
	url := args[0]

	_, err := service.scrapper.RemoveLink(chatID, url)
	if err != nil {
		if errors.Is(err, scrapperdomain.ErrLinkNotFound) || errors.Is(err, scrapperdomain.ErrNotSubscribed) {
			return "You're not tracking this link."
		}
		service.logger.Error("Scrapper RemoveLink failed", slog.String("error", err.Error()))
		return "Something went wrong while untracking the link."
	}

	return fmt.Sprintf("Link %s has been untracked.", url)
}

func (service *CommandService) handleList(chatID int64, args []string) string {
	links, err := service.scrapper.GetTrackedLinks(chatID)
	if err != nil {
		service.logger.Error("Scrapper GetTrackedLinks failed", slog.String("error", err.Error()))
		return "Something went wrong while getting your links."
	}

	if len(links) == 0 {
		return "You have no tracked links."
	}

	sb := strings.Builder{}
	sb.WriteString("Your tracked links:\n")
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

func (service *CommandService) handleStart(chatID int64, args []string) string {
	err := service.scrapper.RegisterChat(chatID)
	if err != nil {
		if errors.Is(err, scrapperdomain.ErrChatAlreadyRegistered) {
			return "Hi again! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
		}
		return "Something went wrong while registering you."
	}
	return "Hi! This bot can track updates on your links, so you won't miss on news! /help for list my commands"
}

func (service *CommandService) handleHelp(chatID int64, args []string) string {
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
