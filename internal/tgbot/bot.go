package tgbot

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"litetask/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	store  *store.Store
	api    *tgbotapi.BotAPI
	chatID int64
}

func Start(ctx context.Context, s *store.Store, token, chatID string) {
	if token == "" || chatID == "" {
		log.Printf("telegram bot is disabled: BOT_TOKEN or BOT_CHAT_ID not set")
		return
	}

	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		log.Printf("telegram bot disabled: invalid BOT_CHAT_ID: %v", err)
		return
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("telegram bot disabled: %v", err)
		return
	}

	b := &Bot{store: s, api: api, chatID: chatIDInt}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := api.GetUpdatesChan(u)
	log.Printf("telegram bot started for chat %d", chatIDInt)

	for {
		select {
		case <-ctx.Done():
			api.StopReceivingUpdates()
			return
		case update, ok := <-updates:
			if !ok {
				return
			}
			if update.Message == nil || update.Message.Chat == nil {
				continue
			}
			if update.Message.Chat.ID != chatIDInt {
				continue
			}
			b.handleMessage(update.Message)
		}
	}
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	cmd, rest := splitCommand(text)
	switch cmd {
	case "/start", "/help":
		reply := "LiteTask бот\n\n" +
			"Команды:\n" +
			"/new [projectId] <название> |комментарий — создать задачу в проекте (по умолчанию Общий)\n" +
			"/status <id> <new|in_progress|done> — сменить статус\n" +
			"/list [projectId] [all] — показать задачи (по умолчанию новые задачи в Общем, all — все статусы, projectId=all — все проекты)\n" +
			"/projects — список проектов\n" +
			"/project <название> — создать проект"
		b.send(reply)
	case "/new", "/add":
		projectID := int64(store.DefaultProjectID)
		content := rest
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			if val, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				projectID = val
				content = strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
			}
		}
		if content == "" {
			b.send("Используй: /new [projectId] <название> |комментарий (комментарий необязателен)")
			return
		}
		title, comment := parseTitleAndComment(content)
		if title == "" {
			b.send("Название задачи не может быть пустым")
			return
		}
		if ok, _ := b.store.ProjectExists(projectID); !ok {
			b.send("Проект не найден")
			return
		}

		t, err := b.store.InsertTask(title, comment, projectID)
		if err != nil {
			log.Printf("bot: failed to insert task: %v", err)
			b.send("Не удалось создать задачу")
			return
		}
		projectName := b.store.LookupProjectName(projectID)
		b.send(fmt.Sprintf("Создана #%d (%s) [%s]: %s", t.ID, projectName, store.StatusTitles[t.Status], t.Title))
	case "/status", "/move":
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			b.send("Используй: /status <id> <new|in_progress|done>")
			return
		}
		taskID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			b.send("ID задачи должен быть числом")
			return
		}
		status := strings.ToLower(strings.TrimSpace(parts[1]))
		t, err := b.store.SetTaskStatus(taskID, status)
		if errors.Is(err, store.ErrInvalidStatus) {
			b.send("Недопустимый статус. Используй new, in_progress или done.")
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			b.send("Задача не найдена")
			return
		}
		if err != nil {
			log.Printf("bot: failed to update status: %v", err)
			b.send("Не удалось обновить статус")
			return
		}
		projectName := b.store.LookupProjectName(t.ProjectID)
		b.send(fmt.Sprintf("Статус задачи #%d (%s) теперь [%s]", t.ID, projectName, store.StatusTitles[t.Status]))
	case "/list":
		projectID := int64(store.DefaultProjectID)
		statusFilter := "new"
		fields := strings.Fields(rest)
		if len(fields) > 0 {
			if strings.ToLower(fields[0]) == "all" {
				projectID = 0
				statusFilter = ""
			} else if val, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
				projectID = val
				if len(fields) > 1 && strings.ToLower(fields[1]) == "all" {
					statusFilter = ""
				}
			}
		}

		tasks, err := b.store.FetchTasks(projectID, statusFilter, nil)
		if err != nil {
			log.Printf("bot: failed to fetch tasks: %v", err)
			b.send("Не удалось получить список задач")
			return
		}
		if len(tasks) == 0 {
			b.send("Задач пока нет")
			return
		}
		var builder strings.Builder
		title := "Задачи:"
		if statusFilter == "new" {
			title = "Новые задачи:"
		}
		if projectID == 0 {
			title += " (все проекты)"
		} else {
			title += fmt.Sprintf(" (проект %s)", b.store.LookupProjectName(projectID))
		}
		if statusFilter == "" {
			title += " (все статусы)"
		}
		builder.WriteString(title + "\n")
		projNames := b.store.ProjectNameMap()
		for _, t := range tasks {
			name := projNames[t.ProjectID]
			fmt.Fprintf(&builder, "#%d (%s) [%s] %s\n", t.ID, name, store.StatusTitles[t.Status], t.Title)
		}
		b.send(builder.String())
	case "/projects":
		projects, err := b.store.ListProjects()
		if err != nil {
			log.Printf("bot: failed to list projects: %v", err)
			b.send("Не удалось получить проекты")
			return
		}
		if len(projects) == 0 {
			b.send("Проектов пока нет")
			return
		}
		var builder strings.Builder
		builder.WriteString("Проекты:\n")
		for _, p := range projects {
			fmt.Fprintf(&builder, "%d — %s\n", p.ID, p.Name)
		}
		b.send(builder.String())
	case "/project":
		if rest == "" {
			b.send("Используй: /project <название>")
			return
		}
		p, err := b.store.CreateProject(strings.TrimSpace(rest))
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				b.send("Проект с таким названием уже существует")
				return
			}
			log.Printf("bot: failed to create project: %v", err)
			b.send("Не удалось создать проект")
			return
		}
		b.send(fmt.Sprintf("Проект создан: #%d %s", p.ID, p.Name))
	default:
		b.send("Неизвестная команда. Отправь /help для подсказки.")
	}
}

func (b *Bot) send(text string) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("failed to send bot message: %v", err)
	}
}

func splitCommand(text string) (string, string) {
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])
	if len(parts) == 1 {
		return cmd, ""
	}
	return cmd, strings.TrimSpace(parts[1])
}

func parseTitleAndComment(input string) (string, string) {
	parts := strings.SplitN(input, "|", 2)
	title := strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		return title, strings.TrimSpace(parts[1])
	}
	return title, ""
}
