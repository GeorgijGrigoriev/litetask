package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"litetask/internal/config"
	"litetask/internal/discordbot"
	"litetask/internal/httpapi"
	"litetask/internal/store"
	"litetask/internal/tgbot"
)

const defaultAddr = ":8080"

func main() {
	dbPath := config.EnvOrDefault("DB_PATH", store.DefaultDBPath)
	st, err := store.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := st.Close(); err != nil {
			slog.Error("failed to close store", "err", err)
		}
	}()

	secret, err := loadSecret()
	if err != nil {
		slog.Error("failed to load auth secret", "err", err)
		os.Exit(1)
	}

	allowRegistration := config.EnvOrDefault("ALLOW_REGISTRATION", "true") != "false"

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if config.EnvOrDefault("TELEGRAM_ENABLED", "true") != "false" {
		go tgbot.Start(ctx, st, strings.TrimSpace(os.Getenv("BOT_TOKEN")), strings.TrimSpace(os.Getenv("BOT_CHAT_ID")))
	}
	if config.EnvOrDefault("DISCORD_ENABLED", "true") != "false" {
		go discordbot.Start(ctx, st, strings.TrimSpace(os.Getenv("DISCORD_TOKEN")), strings.TrimSpace(os.Getenv("DISCORD_CHANNEL_ID")), strings.TrimSpace(os.Getenv("DISCORD_GUILD_ID")))
	}

	listenAddr := defaultAddr
	if port := os.Getenv("PORT"); port != "" {
		listenAddr = port
	}

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      httpapi.New(st, secret, allowRegistration, "web/dist").Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server started", "addr", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}

func loadSecret() ([]byte, error) {
	if val := os.Getenv("AUTH_SECRET"); val != "" {
		decoded, err := base64.StdEncoding.DecodeString(val)
		if err == nil && len(decoded) >= 32 {
			return decoded, nil
		}
		if len(val) >= 32 {
			return []byte(val), nil
		}
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	slog.Warn("generated random auth secret; set AUTH_SECRET to persist sessions")
	return secret, nil
}
