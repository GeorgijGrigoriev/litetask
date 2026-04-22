package botcore

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"litetask/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	t.Setenv("ADMIN_EMAIL", "admin@example.com")
	t.Setenv("ADMIN_PASSWORD", "secret123")
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestHandleHelp(t *testing.T) {
	ctx := context.Background()
	c := New(openTestStore(t), "")
	resp := c.Handle(ctx, "/help", "")
	if !strings.Contains(resp, "/new") || !strings.Contains(resp, "/list") {
		t.Fatalf("unexpected help text: %q", resp)
	}
}

func TestHandleStart(t *testing.T) {
	ctx := context.Background()
	c := New(openTestStore(t), "")
	if c.Handle(ctx, "/start", "") != c.Handle(ctx, "/help", "") {
		t.Fatal("/start and /help should return the same text")
	}
}

func TestHandleUnknown(t *testing.T) {
	ctx := context.Background()
	c := New(openTestStore(t), "")
	resp := c.Handle(ctx, "/unknown", "")
	if !strings.Contains(resp, "Неизвестная команда") {
		t.Fatalf("unexpected response: %q", resp)
	}
}

func TestHandleNewAndList(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	c := New(st, "")

	resp := c.Handle(ctx, "/new", "")
	if !strings.Contains(resp, "Используй") {
		t.Fatalf("expected usage hint, got %q", resp)
	}

	resp = c.Handle(ctx, "/new", "My Task |some description")
	if !strings.Contains(resp, "Создана #") {
		t.Fatalf("expected task creation, got %q", resp)
	}

	resp = c.Handle(ctx, "/list", "")
	if !strings.Contains(resp, "Новые задачи:") {
		t.Fatalf("expected task list, got %q", resp)
	}
	if !strings.Contains(resp, "My Task") {
		t.Fatalf("expected task title in list, got %q", resp)
	}
}

func TestHandleNewInvalidProject(t *testing.T) {
	ctx := context.Background()
	c := New(openTestStore(t), "")
	resp := c.Handle(ctx, "/new", "9999 Task")
	if !strings.Contains(resp, "Проект не найден") {
		t.Fatalf("expected invalid project, got %q", resp)
	}
}

func TestHandleNewEmptyTitle(t *testing.T) {
	ctx := context.Background()
	c := New(openTestStore(t), "")
	resp := c.Handle(ctx, "/new", "|description only")
	if !strings.Contains(resp, "Название задачи не может быть пустым") {
		t.Fatalf("expected empty title error, got %q", resp)
	}
}

func TestHandleStatus(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	c := New(st, "")

	c.Handle(ctx, "/new", "Status Test Task")
	tasks, _ := st.FetchTasks(ctx, store.DefaultProjectID, "", nil)
	if len(tasks) == 0 {
		t.Fatal("expected task to be created")
	}
	id := strconv.FormatInt(tasks[0].ID, 10)

	resp := c.Handle(ctx, "/status", id+" done")
	if !strings.Contains(resp, "Статус задачи #") {
		t.Fatalf("expected status update, got %q", resp)
	}

	resp = c.Handle(ctx, "/status", "999999 done")
	if !strings.Contains(resp, "Задача не найдена") {
		t.Fatalf("expected not found, got %q", resp)
	}

	resp = c.Handle(ctx, "/status", id+" invalid_status")
	if !strings.Contains(resp, "Недопустимый статус") {
		t.Fatalf("expected invalid status, got %q", resp)
	}

	resp = c.Handle(ctx, "/status", "")
	if !strings.Contains(resp, "Используй") {
		t.Fatalf("expected usage hint, got %q", resp)
	}

	resp = c.Handle(ctx, "/status", "abc done")
	if !strings.Contains(resp, "числом") {
		t.Fatalf("expected numeric id error, got %q", resp)
	}
}

func TestHandleProjects(t *testing.T) {
	ctx := context.Background()
	c := New(openTestStore(t), "")

	resp := c.Handle(ctx, "/projects", "")
	if !strings.Contains(resp, "Общий") {
		t.Fatalf("expected default project, got %q", resp)
	}

	resp = c.Handle(ctx, "/project", "")
	if !strings.Contains(resp, "Используй") {
		t.Fatalf("expected usage hint, got %q", resp)
	}

	resp = c.Handle(ctx, "/project", "Backend")
	if !strings.Contains(resp, "Проект создан") {
		t.Fatalf("expected project created, got %q", resp)
	}

	resp = c.Handle(ctx, "/projects", "")
	if !strings.Contains(resp, "Backend") {
		t.Fatalf("expected Backend in project list, got %q", resp)
	}

	resp = c.Handle(ctx, "/project", "Backend")
	if !strings.Contains(resp, "уже существует") {
		t.Fatalf("expected duplicate error, got %q", resp)
	}
}

func TestHandleListAllProjects(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	c := New(st, "")

	c.Handle(ctx, "/new", "Task A")
	c.Handle(ctx, "/project", "Other")
	projects, _ := st.ListProjects(ctx)
	var otherID int64
	for _, p := range projects {
		if p.Name == "Other" {
			otherID = p.ID
		}
	}
	c.Handle(ctx, "/new", strconv.FormatInt(otherID, 10)+" Task B")

	resp := c.Handle(ctx, "/list", "all all")
	if !strings.Contains(resp, "Task A") || !strings.Contains(resp, "Task B") {
		t.Fatalf("expected both tasks in list, got %q", resp)
	}
}

func TestSplitCommand(t *testing.T) {
	cases := []struct{ input, cmd, args string }{
		{"/help", "/help", ""},
		{"/new Task title", "/new", "Task title"},
		{"/STATUS 1 done", "/status", "1 done"},
	}
	for _, tc := range cases {
		cmd, args := SplitCommand(tc.input)
		if cmd != tc.cmd || args != tc.args {
			t.Errorf("SplitCommand(%q) = (%q, %q), want (%q, %q)", tc.input, cmd, args, tc.cmd, tc.args)
		}
	}
}

func TestParseTitleAndDescription(t *testing.T) {
	title, desc := ParseTitleAndDescription("My Task | Some desc")
	if title != "My Task" || desc != "Some desc" {
		t.Errorf("got (%q, %q)", title, desc)
	}

	title, desc = ParseTitleAndDescription("Only Title")
	if title != "Only Title" || desc != "" {
		t.Errorf("got (%q, %q)", title, desc)
	}
}
