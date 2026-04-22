package tgbot

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"litetask/internal/botcore"
	"litetask/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type fakeSender struct {
	messages []string
}

func (f *fakeSender) Send(msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	switch v := msg.(type) {
	case tgbotapi.MessageConfig:
		f.messages = append(f.messages, v.Text)
		return tgbotapi.Message{Text: v.Text}, nil
	default:
		f.messages = append(f.messages, "")
		return tgbotapi.Message{}, nil
	}
}

func (f *fakeSender) last() string {
	if len(f.messages) == 0 {
		return ""
	}
	return f.messages[len(f.messages)-1]
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "secret123")
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st
}

func newMessage(text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Text: text,
		Chat: &tgbotapi.Chat{ID: 1},
	}
}

func newTestBot(t *testing.T) (*Bot, *fakeSender) {
	t.Helper()
	st := openTestStore(t)
	sender := &fakeSender{}
	return &Bot{commander: botcore.New(st, ""), sender: sender, chatID: 1}, sender
}

func TestHandleMessageNewStatusList(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	sender := &fakeSender{}
	b := &Bot{commander: botcore.New(st, ""), sender: sender, chatID: 1}

	b.handleMessage(ctx, newMessage("/new Task one |desc"))
	if !strings.Contains(sender.last(), "Создана #") {
		t.Fatalf("expected create response, got %q", sender.last())
	}

	tasks, err := st.FetchTasks(ctx, store.DefaultProjectID, "", nil)
	if err != nil {
		t.Fatalf("fetch tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	taskID := tasks[0].ID

	b.handleMessage(ctx, newMessage("/status 999999 done"))
	if !strings.Contains(sender.last(), "Задача не найдена") {
		t.Fatalf("expected not found, got %q", sender.last())
	}

	b.handleMessage(ctx, newMessage("/status "+strconv.FormatInt(taskID, 10)+" done"))
	if !strings.Contains(sender.last(), "Статус задачи #") {
		t.Fatalf("expected status response, got %q", sender.last())
	}

	b.handleMessage(ctx, newMessage("/list all all"))
	if !strings.Contains(sender.last(), "Задачи:") {
		t.Fatalf("expected list response, got %q", sender.last())
	}
	if !strings.Contains(sender.last(), "Task one") {
		t.Fatalf("expected list to include task title")
	}
}

func TestHandleMessageProjects(t *testing.T) {
	ctx := context.Background()
	b, sender := newTestBot(t)

	b.handleMessage(ctx, newMessage("/project Backend"))
	if !strings.Contains(sender.last(), "Проект создан") {
		t.Fatalf("expected project created, got %q", sender.last())
	}

	b.handleMessage(ctx, newMessage("/projects"))
	if !strings.Contains(sender.last(), "Backend") {
		t.Fatalf("expected project list to include Backend, got %q", sender.last())
	}
}

func TestHandleMessageInvalidProject(t *testing.T) {
	ctx := context.Background()
	b, sender := newTestBot(t)

	b.handleMessage(ctx, newMessage("/new 9999 Task"))
	if !strings.Contains(sender.last(), "Проект не найден") {
		t.Fatalf("expected invalid project, got %q", sender.last())
	}
}

func TestHandleMessageHelp(t *testing.T) {
	ctx := context.Background()
	b, sender := newTestBot(t)

	b.handleMessage(ctx, newMessage("/help"))
	if !strings.Contains(sender.last(), "/new") {
		t.Fatalf("expected help text, got %q", sender.last())
	}
}

func TestHandleMessageUnknownCommand(t *testing.T) {
	ctx := context.Background()
	b, sender := newTestBot(t)

	b.handleMessage(ctx, newMessage("/foo bar"))
	if !strings.Contains(sender.last(), "Неизвестная команда") {
		t.Fatalf("expected unknown command response, got %q", sender.last())
	}
}

func TestHandleEmptyMessage(t *testing.T) {
	ctx := context.Background()
	b, sender := newTestBot(t)

	b.handleMessage(ctx, newMessage("   "))
	if len(sender.messages) != 0 {
		t.Fatalf("expected no response for empty message, got %q", sender.last())
	}
}
