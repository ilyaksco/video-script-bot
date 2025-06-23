package main

import (
	"log"
	"video-script-bot/internal/ai"
	"video-script-bot/internal/apikeys"
	"video-script-bot/internal/bot"
	"video-script-bot/internal/config"
	"video-script-bot/internal/i18n"
	"video-script-bot/internal/storage"
)

func main() {
	cfg := config.LoadConfig()

	localizer := i18n.NewLocalizer(cfg.DefaultLang)
	
	db, err := storage.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("FATAL: Could not initialize database: %v", err)
	}

	geminiKeyManager, err := apikeys.NewManager(cfg.GeminiAPIKeys)
	if err != nil {
		log.Printf("WARNING: Could not initialize Gemini Key Manager: %v. Gemini features will be disabled.", err)
	}

	elevenlabsKeyManager, err := apikeys.NewManager(cfg.ElevenLabsAPIKeys)
	if err != nil {
		log.Printf("WARNING: Could not initialize ElevenLabs Key Manager: %v. TTS features will be disabled.", err)
	}

	geminiService := ai.NewGeminiService(geminiKeyManager)
	
	elevenlabsService, err := ai.NewElevenLabsService(elevenlabsKeyManager, cfg.ElevenLabsModelID)
	if err != nil {
		log.Fatalf("FATAL: Could not initialize ElevenLabs service: %v", err)
	}

	telegramBot, err := bot.New(cfg, localizer, db, geminiService, elevenlabsService)
	if err != nil {
		log.Fatalf("FATAL: Could not initialize bot: %v", err)
	}

	log.Println("Bot initialized successfully with API Key Rotation. Starting to listen for updates...")

	telegramBot.Start()
}
