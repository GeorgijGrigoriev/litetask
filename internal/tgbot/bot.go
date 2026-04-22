package tgbot

import (
	"context"
	"log"
	"strconv"
	"strings"

	"litetask/internal/botcore"
	"litetask/internal/store"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	commander *botcore.Commander
	sender    tgSender
	chatID    int64
}

type tgSender interface {
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
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

	b := &Bot{commander: botcore.New(s, "@"+api.Self.UserName), sender: api, chatID: chatIDInt}

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
			b.handleMessage(ctx, update.Message)
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}
	cmd, args := botcore.SplitCommand(text)
	b.send(b.commander.Handle(ctx, cmd, args))
}

func (b *Bot) send(text string) {
	msg := tgbotapi.NewMessage(b.chatID, text)
	if _, err := b.sender.Send(msg); err != nil {
		log.Printf("failed to send telegram message: %v", err)
	}
}
