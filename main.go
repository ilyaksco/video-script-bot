package main

import (
	"context"
	"log"
	"video-script-bot/internal/ai"
	"video-script-bot/internal/bot"
	"video-script-bot/internal/config"
	"video-script-bot/internal/i18n"
	"video-script-bot/internal/storage"
)

func main() {
	ctx := context.Background()
	cfg := config.LoadConfig()

	localizer := i18n.NewLocalizer(cfg.DefaultLang)
	
	db, err := storage.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("FATAL: Could not initialize database: %v", err)
	}

	geminiService, err := ai.NewGeminiService(ctx, cfg.GeminiAPIKey)
	if err != nil {
		log.Fatalf("FATAL: Could not initialize Gemini service: %v", err)
	}

	elevenlabsService, err := ai.NewElevenLabsService(cfg.ElevenLabsAPIKey, cfg.ElevenLabsModelID)
	if err != nil {
		log.Fatalf("FATAL: Could not initialize ElevenLabs service: %v", err)
	}

	telegramBot, err := bot.New(cfg, localizer, db, geminiService, elevenlabsService)
	if err != nil {
		log.Fatalf("FATAL: Could not initialize bot: %v", err)
	}

	log.Println("Bot initialized successfully with SQLite database. Starting to listen for updates...")

	telegramBot.Start()
}
