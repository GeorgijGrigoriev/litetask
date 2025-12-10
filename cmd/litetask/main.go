package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strings"

	"litetask/internal/config"
	"litetask/internal/httpapi"
	"litetask/internal/store"
	"litetask/internal/tgbot"
)

const defaultAddr = ":8080"

func main() {
	dbPath := config.EnvOrDefault("DB_PATH", store.DefaultDBPath)
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer st.Close()

	secret, err := loadSecret()
	if err != nil {
		log.Fatalf("failed to load auth secret: %v", err)
	}

	allowRegistration := config.EnvOrDefault("ALLOW_REGISTRATION", "true") != "false"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go tgbot.Start(ctx, st, strings.TrimSpace(os.Getenv("BOT_TOKEN")), strings.TrimSpace(os.Getenv("BOT_CHAT_ID")))

	server := httpapi.New(st, secret, allowRegistration, "web/dist")

	log.Printf("listening on %s", defaultAddr)
	if err := http.ListenAndServe(defaultAddr, server.Routes()); err != nil {
		log.Fatalf("server error: %v", err)
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
	log.Printf("generated random auth secret; set AUTH_SECRET to persist sessions")
	return secret, nil
}
