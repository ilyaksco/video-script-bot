package config

import (
	"log"
	"os"
	"strconv"
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
	StorageChannelID  int64
	SaweriaLink       string
	BuyMeACoffeeLink  string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	geminiKeys := strings.Split(getEnv("GEMINI_API_KEYS", "", true), ",")
	elevenKeys := strings.Split(getEnv("ELEVENLABS_API_KEYS", "", true), ",")
	token := getEnv("TELEGRAM_BOT_TOKEN", "", true)
	storageIDStr := getEnv("STORAGE_CHANNEL_ID", "", true)

	storageID, err := strconv.ParseInt(storageIDStr, 10, 64)
	if err != nil {
		log.Fatalf("FATAL: Invalid STORAGE_CHANNEL_ID. It must be a valid integer. Error: %v", err)
	}

	return &Config{
		TelegramBotToken:  token,
		GeminiAPIKeys:     geminiKeys,
		ElevenLabsAPIKeys: elevenKeys,
		ElevenLabsModelID: getEnv("ELEVENLABS_MODEL_ID", "eleven_multilingual_v2", false),
		DefaultLang:       getEnv("DEFAULT_LANG", "en", false),
		DatabasePath:      getEnv("DATABASE_PATH", "./bot_data.db", false),
		StorageChannelID:  storageID,
		SaweriaLink:       getEnv("SAWERIA_LINK", "", false),
		BuyMeACoffeeLink:  getEnv("BUYMEACOFFEE_LINK", "", false),
	}
}

func getEnv(key, fallback string, required bool) string {
	value, exists := os.LookupEnv(key)

	if !exists {
		if required {
			log.Fatalf("FATAL: Required environment variable %s is not set.", key)
		}
		if fallback != "" {
			return fallback
		}
		return ""
	}

	if required && value == "" {
		log.Fatalf("FATAL: Required environment variable %s is set but empty.", key)
	}

	return value
}
