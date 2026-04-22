package discordbot

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"litetask/internal/botcore"
	"litetask/internal/store"

	"github.com/bwmarrin/discordgo"
)

type fakeSession struct {
	responses []string
}

func (f *fakeSession) InteractionRespond(i *discordgo.Interaction, r *discordgo.InteractionResponse, opts ...discordgo.RequestOption) error {
	f.responses = append(f.responses, r.Data.Content)
	return nil
}

func (f *fakeSession) last() string {
	if len(f.responses) == 0 {
		return ""
	}
	return f.responses[len(f.responses)-1]
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
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func newTestBot(t *testing.T) (*Bot, *fakeSession) {
	t.Helper()
	st := openTestStore(t)
	s := &fakeSession{}
	b := &Bot{commander: botcore.New(st, ""), channelID: "ch1"}
	return b, s
}

func interaction(channelID, cmdName string, opts ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:      discordgo.InteractionApplicationCommand,
		ChannelID: channelID,
		Data: discordgo.ApplicationCommandInteractionData{
			Name:    cmdName,
			Options: opts,
		},
	}}
}

func strOpt(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func intOpt(name string, value int64) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionInteger,
		Value: float64(value),
	}
}

func boolOpt(name string, value bool) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionBoolean,
		Value: value,
	}
}

func TestHandleHelp(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)
	b.handleInteraction(ctx, s, interaction("ch1", "help"))
	if !strings.Contains(s.last(), "/new") {
		t.Fatalf("expected help text, got %q", s.last())
	}
}

func TestHandleNew(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)

	b.handleInteraction(ctx, s, interaction("ch1", "new", strOpt("title", "My Task"), strOpt("description", "desc")))
	if !strings.Contains(s.last(), "Создана #") {
		t.Fatalf("expected task creation, got %q", s.last())
	}
	if !strings.Contains(s.last(), "My Task") {
		t.Fatalf("expected task title in response, got %q", s.last())
	}
}

func TestHandleNewInvalidProject(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)
	b.handleInteraction(ctx, s, interaction("ch1", "new", strOpt("title", "Task"), intOpt("project", 9999)))
	if !strings.Contains(s.last(), "Проект не найден") {
		t.Fatalf("expected invalid project, got %q", s.last())
	}
}

func TestHandleStatus(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)

	b.handleInteraction(ctx, s, interaction("ch1", "new", strOpt("title", "Status Task")))
	b.handleInteraction(ctx, s, interaction("ch1", "status", intOpt("id", 1), strOpt("status", "done")))
	if !strings.Contains(s.last(), "Статус задачи #") {
		t.Fatalf("expected status update, got %q", s.last())
	}
}

func TestHandleStatusNotFound(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)
	b.handleInteraction(ctx, s, interaction("ch1", "status", intOpt("id", 99999), strOpt("status", "done")))
	if !strings.Contains(s.last(), "Задача не найдена") {
		t.Fatalf("expected not found, got %q", s.last())
	}
}

func TestHandleList(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)

	b.handleInteraction(ctx, s, interaction("ch1", "list"))
	if !strings.Contains(s.last(), "Задач пока нет") {
		t.Fatalf("expected empty list, got %q", s.last())
	}

	b.handleInteraction(ctx, s, interaction("ch1", "new", strOpt("title", "Task A")))
	b.handleInteraction(ctx, s, interaction("ch1", "list"))
	if !strings.Contains(s.last(), "Task A") {
		t.Fatalf("expected Task A in list, got %q", s.last())
	}
}

func TestHandleListAllProjects(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)

	b.handleInteraction(ctx, s, interaction("ch1", "new", strOpt("title", "Task A")))
	b.handleInteraction(ctx, s, interaction("ch1", "list", boolOpt("all_projects", true), boolOpt("all_statuses", true)))
	if !strings.Contains(s.last(), "Task A") {
		t.Fatalf("expected Task A in all-projects list, got %q", s.last())
	}
}

func TestHandleProjects(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)

	b.handleInteraction(ctx, s, interaction("ch1", "project", strOpt("name", "Backend")))
	if !strings.Contains(s.last(), "Проект создан") {
		t.Fatalf("expected project created, got %q", s.last())
	}

	b.handleInteraction(ctx, s, interaction("ch1", "projects"))
	if !strings.Contains(s.last(), "Backend") {
		t.Fatalf("expected Backend in projects, got %q", s.last())
	}
}

func TestIgnoresWrongChannel(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)
	b.handleInteraction(ctx, s, interaction("wrong_channel", "help"))
	if len(s.responses) != 0 {
		t.Fatal("expected wrong channel to be ignored")
	}
}

func TestIgnoresNonCommandInteraction(t *testing.T) {
	ctx := context.Background()
	b, s := newTestBot(t)
	b.handleInteraction(ctx, s, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:      discordgo.InteractionMessageComponent,
		ChannelID: "ch1",
	}})
	if len(s.responses) != 0 {
		t.Fatal("expected non-command interaction to be ignored")
	}
}

func TestBuildArgs(t *testing.T) {
	cases := []struct {
		name string
		opts []*discordgo.ApplicationCommandInteractionDataOption
		want string
	}{
		{"new", []*discordgo.ApplicationCommandInteractionDataOption{strOpt("title", "My Task")}, "1 My Task"},
		{"new", []*discordgo.ApplicationCommandInteractionDataOption{strOpt("title", "T"), strOpt("description", "d"), intOpt("project", 2)}, "2 T|d"},
		{"status", []*discordgo.ApplicationCommandInteractionDataOption{intOpt("id", 5), strOpt("status", "done")}, "5 done"},
		{"list", nil, ""},
		{"list", []*discordgo.ApplicationCommandInteractionDataOption{boolOpt("all_projects", true)}, "all"},
		{"list", []*discordgo.ApplicationCommandInteractionDataOption{intOpt("project", 2), boolOpt("all_statuses", true)}, "2 all"},
		{"list", []*discordgo.ApplicationCommandInteractionDataOption{boolOpt("all_statuses", true)}, "1 all"},
		{"project", []*discordgo.ApplicationCommandInteractionDataOption{strOpt("name", "Foo")}, "Foo"},
	}
	for _, tc := range cases {
		got := buildArgs(tc.name, tc.opts)
		if got != tc.want {
			t.Errorf("buildArgs(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
