package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken  string
	GeminiAPIKeys     []string
	ElevenLabsAPIKeys []string
	ElevenLabsModelID string
	DefaultLang       string
	DatabasePath      string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	geminiKeys := strings.Split(getEnv("GEMINI_API_KEYS", ""), ",")
	elevenKeys := strings.Split(getEnv("ELEVENLABS_API_KEYS", ""), ",")

	return &Config{
		TelegramBotToken:  getEnv("TELEGRAM_BOT_TOKEN", ""),
		GeminiAPIKeys:     geminiKeys,
		ElevenLabsAPIKeys: elevenKeys,
		ElevenLabsModelID: getEnv("ELEVENLABS_MODEL_ID", "eleven_multilingual_v2"),
		DefaultLang:       getEnv("DEFAULT_LANG", "en"),
		DatabasePath:      getEnv("DATABASE_PATH", "./bot_data.db"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if fallback != "" {
		return fallback
	}
	log.Fatalf("FATAL: Environment variable %s is not set.", key)
	return ""
}
