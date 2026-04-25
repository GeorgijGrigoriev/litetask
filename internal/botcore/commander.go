package botcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"litetask/internal/store"
)

// Commander handles bot commands and returns text responses.
// It is platform-agnostic and shared by all bot integrations.
type Commander struct {
	store   store.Storer
	botName string
}

func New(s store.Storer, botName string) *Commander {
	return &Commander{store: s, botName: botName}
}

func (c *Commander) Handle(ctx context.Context, cmd, args string) string {
	switch cmd {
	case "/start", "/help":
		return helpText()
	case "/new", "/add":
		return c.handleNew(ctx, args)
	case "/status", "/move":
		return c.handleStatus(ctx, args)
	case "/list":
		return c.handleList(ctx, args)
	case "/projects":
		return c.handleProjects(ctx)
	case "/project":
		return c.handleProject(ctx, args)
	default:
		return "Неизвестная команда. Отправь /help для подсказки."
	}
}

func helpText() string {
	return "LiteTask бот\n\n" +
		"Команды:\n" +
		"/new [projectId] <название> |описание — создать задачу в проекте (по умолчанию Общий)\n" +
		"/status <id> <new|in_progress|done> — сменить статус\n" +
		"/list [projectId] [all] — показать задачи (по умолчанию новые задачи в Общем, all — все статусы, projectId=all — все проекты)\n" +
		"/projects — список проектов\n" +
		"/project <название> — создать проект"
}

func (c *Commander) handleNew(ctx context.Context, args string) string {
	projectID := int64(store.DefaultProjectID)
	content := args
	fields := strings.Fields(args)
	if len(fields) > 0 {
		if val, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
			projectID = val
			content = strings.TrimSpace(strings.TrimPrefix(args, fields[0]))
		}
	}
	if content == "" {
		return "Используй: /new [projectId] <название> |описание (описание необязательно)"
	}
	title, description := ParseTitleAndDescription(content)
	if title == "" {
		return "Название задачи не может быть пустым"
	}
	if ok, _ := c.store.ProjectExists(ctx, projectID); !ok {
		return "Проект не найден"
	}
	t, err := c.store.InsertTask(ctx, title, description, c.botName, projectID, 0, "medium")
	if err != nil {
		log.Printf("bot: failed to insert task: %v", err)
		return "Не удалось создать задачу"
	}
	projectName := c.store.LookupProjectName(ctx, projectID)
	return fmt.Sprintf("Создана #%d (%s) [%s]: %s", t.ID, projectName, store.StatusTitles[t.Status], t.Title)
}

func (c *Commander) handleStatus(ctx context.Context, args string) string {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return "Используй: /status <id> <new|in_progress|done>"
	}
	taskID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "ID задачи должен быть числом"
	}
	status := strings.ToLower(strings.TrimSpace(parts[1]))
	t, err := c.store.SetTaskStatus(ctx, taskID, status)
	if errors.Is(err, store.ErrInvalidStatus) {
		return "Недопустимый статус. Используй new, in_progress или done."
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "Задача не найдена"
	}
	if err != nil {
		log.Printf("bot: failed to update status: %v", err)
		return "Не удалось обновить статус"
	}
	projectName := c.store.LookupProjectName(ctx, t.ProjectID)
	return fmt.Sprintf("Статус задачи #%d (%s) теперь [%s]", t.ID, projectName, store.StatusTitles[t.Status])
}

func (c *Commander) handleList(ctx context.Context, args string) string {
	projectID := int64(store.DefaultProjectID)
	statusFilter := "new"
	fields := strings.Fields(args)
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
	tasks, err := c.store.FetchTasks(ctx, projectID, statusFilter, nil)
	if err != nil {
		log.Printf("bot: failed to fetch tasks: %v", err)
		return "Не удалось получить список задач"
	}
	if len(tasks) == 0 {
		return "Задач пока нет"
	}
	var b strings.Builder
	title := "Задачи:"
	if statusFilter == "new" {
		title = "Новые задачи:"
	}
	if projectID == 0 {
		title += " (все проекты)"
	} else {
		title += fmt.Sprintf(" (проект %s)", c.store.LookupProjectName(ctx, projectID))
	}
	if statusFilter == "" {
		title += " (все статусы)"
	}
	b.WriteString(title + "\n")
	projNames := c.store.ProjectNameMap(ctx)
	for _, t := range tasks {
		name := projNames[t.ProjectID]
		fmt.Fprintf(&b, "#%d (%s) [%s] %s\n", t.ID, name, store.StatusTitles[t.Status], t.Title)
	}
	return b.String()
}

func (c *Commander) handleProjects(ctx context.Context) string {
	projects, err := c.store.ListProjects(ctx)
	if err != nil {
		log.Printf("bot: failed to list projects: %v", err)
		return "Не удалось получить проекты"
	}
	if len(projects) == 0 {
		return "Проектов пока нет"
	}
	var b strings.Builder
	b.WriteString("Проекты:\n")
	for _, p := range projects {
		fmt.Fprintf(&b, "%d — %s\n", p.ID, p.Name)
	}
	return b.String()
}

func (c *Commander) handleProject(ctx context.Context, args string) string {
	if args == "" {
		return "Используй: /project <название>"
	}
	p, err := c.store.CreateProject(ctx, strings.TrimSpace(args))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return "Проект с таким названием уже существует"
		}
		log.Printf("bot: failed to create project: %v", err)
		return "Не удалось создать проект"
	}
	return fmt.Sprintf("Проект создан: #%d %s", p.ID, p.Name)
}

// SplitCommand splits "/cmd rest" into ("cmd", "rest").
func SplitCommand(text string) (string, string) {
	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])
	if len(parts) == 1 {
		return cmd, ""
	}
	return cmd, strings.TrimSpace(parts[1])
}

// ParseTitleAndDescription splits "title|description" into two parts.
func ParseTitleAndDescription(input string) (string, string) {
	parts := strings.SplitN(input, "|", 2)
	title := strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		return title, strings.TrimSpace(parts[1])
	}
	return title, ""
}
