package discordbot

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"litetask/internal/botcore"
	"litetask/internal/store"

	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	commander *botcore.Commander
	channelID string
}

// discordSession is the subset used by handleInteraction, enabling test fakes.
type discordSession interface {
	InteractionRespond(i *discordgo.Interaction, r *discordgo.InteractionResponse, opts ...discordgo.RequestOption) error
}

var appCommands = []*discordgo.ApplicationCommand{
	{Name: "help", Description: "Показать список команд"},
	{
		Name:        "new",
		Description: "Создать задачу",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "title", Description: "Название задачи", Required: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "description", Description: "Описание (необязательно)", Required: false},
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "project", Description: "ID проекта (по умолчанию Общий)", Required: false},
		},
	},
	{
		Name:        "status",
		Description: "Сменить статус задачи",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "id", Description: "ID задачи", Required: true},
			{
				Type: discordgo.ApplicationCommandOptionString, Name: "status", Description: "Новый статус", Required: true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "new", Value: "new"},
					{Name: "in_progress", Value: "in_progress"},
					{Name: "done", Value: "done"},
				},
			},
		},
	},
	{
		Name:        "list",
		Description: "Показать задачи",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionInteger, Name: "project", Description: "ID проекта", Required: false},
			{Type: discordgo.ApplicationCommandOptionBoolean, Name: "all_projects", Description: "Все проекты", Required: false},
			{Type: discordgo.ApplicationCommandOptionBoolean, Name: "all_statuses", Description: "Все статусы", Required: false},
		},
	},
	{Name: "projects", Description: "Список проектов"},
	{
		Name:        "project",
		Description: "Создать проект",
		Options: []*discordgo.ApplicationCommandOption{
			{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "Название проекта", Required: true},
		},
	},
}

func Start(ctx context.Context, s *store.Store, token, channelID, guildID string) {
	if token == "" || channelID == "" {
		log.Printf("discord bot is disabled: DISCORD_TOKEN or DISCORD_CHANNEL_ID not set")
		return
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Printf("discord bot disabled: %v", err)
		return
	}
	session.Identify.Intents = discordgo.IntentsGuilds

	b := &Bot{channelID: channelID}
	bctx := ctx
	session.AddHandler(func(sess *discordgo.Session, i *discordgo.InteractionCreate) {
		b.handleInteraction(bctx, sess, i)
	})

	if err := session.Open(); err != nil {
		log.Printf("discord bot disabled: failed to open session: %v", err)
		return
	}

	botName := ""
	if session.State != nil && session.State.User != nil {
		botName = "@" + session.State.User.Username
	}
	b.commander = botcore.New(s, botName)

	registered, err := registerCommands(session, guildID)
	if err != nil {
		log.Printf("discord bot: failed to register commands: %v", err)
		_ = session.Close()
		return
	}
	log.Printf("discord bot started for channel %s (%d commands registered)", channelID, len(registered))

	<-ctx.Done()
	deleteCommands(session, guildID, registered)
	if err := session.Close(); err != nil {
		log.Printf("discord bot: close error: %v", err)
	}
}

func (b *Bot) handleInteraction(ctx context.Context, s discordSession, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ChannelID != b.channelID {
		return
	}
	data := i.ApplicationCommandData()
	args := buildArgs(data.Name, data.Options)
	reply := b.commander.Handle(ctx, "/"+data.Name, args)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: reply},
	}); err != nil {
		log.Printf("discord bot: failed to respond: %v", err)
	}
}

func buildArgs(name string, opts []*discordgo.ApplicationCommandInteractionDataOption) string {
	get := func(key string) *discordgo.ApplicationCommandInteractionDataOption {
		for _, o := range opts {
			if o.Name == key {
				return o
			}
		}
		return nil
	}

	switch name {
	case "new":
		projectID := int64(store.DefaultProjectID)
		if p := get("project"); p != nil {
			projectID = p.IntValue()
		}
		title := get("title").StringValue()
		args := strconv.FormatInt(projectID, 10) + " " + title
		if d := get("description"); d != nil && d.StringValue() != "" {
			args += "|" + d.StringValue()
		}
		return args
	case "status":
		return strconv.FormatInt(get("id").IntValue(), 10) + " " + get("status").StringValue()
	case "list":
		allProjects := get("all_projects") != nil && get("all_projects").BoolValue()
		allStatuses := get("all_statuses") != nil && get("all_statuses").BoolValue()
		projectOpt := get("project")

		var projectPart string
		switch {
		case allProjects:
			projectPart = "all"
		case projectOpt != nil:
			projectPart = strconv.FormatInt(projectOpt.IntValue(), 10)
		}

		if projectPart == "" && !allStatuses {
			return ""
		}
		if projectPart == "" {
			projectPart = strconv.FormatInt(int64(store.DefaultProjectID), 10)
		}
		if allStatuses {
			return projectPart + " all"
		}
		return projectPart
	case "project":
		return get("name").StringValue()
	}
	return ""
}

func registerCommands(session *discordgo.Session, guildID string) ([]*discordgo.ApplicationCommand, error) {
	appID := session.State.User.ID
	registered := make([]*discordgo.ApplicationCommand, 0, len(appCommands))
	for _, cmd := range appCommands {
		c, err := session.ApplicationCommandCreate(appID, guildID, cmd)
		if err != nil {
			return registered, fmt.Errorf("register command %q: %w", cmd.Name, err)
		}
		registered = append(registered, c)
	}
	return registered, nil
}

func deleteCommands(session *discordgo.Session, guildID string, cmds []*discordgo.ApplicationCommand) {
	appID := session.State.User.ID
	for _, cmd := range cmds {
		if err := session.ApplicationCommandDelete(appID, guildID, cmd.ID); err != nil {
			log.Printf("discord bot: failed to delete command %q: %v", cmd.Name, err)
		}
	}
}
