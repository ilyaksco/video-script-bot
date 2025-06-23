package bot

import (
	"io"
	"log"
	"net/http"
	"video-script-bot/internal/ai"
	"video-script-bot/internal/config"
	"video-script-bot/internal/models"
	"video-script-bot/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Bot struct {
	api               *tgbotapi.BotAPI
	cfg               *config.Config
	localizer         *i18n.Localizer
	db                *storage.Storage
	geminiService     *ai.GeminiService
	elevenlabsService *ai.ElevenLabsService
}

func New(cfg *config.Config, localizer *i18n.Localizer, db *storage.Storage, geminiService *ai.GeminiService, elevenlabsService *ai.ElevenLabsService) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		return nil, err
	}

	api.Debug = false
	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:               api,
		cfg:               cfg,
		localizer:         localizer,
		db:                db,
		geminiService:     geminiService,
		elevenlabsService: elevenlabsService,
	}, nil
}

func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		var userID int64
		var chatID int64
		var isCallback bool

		if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
			chatID = update.CallbackQuery.Message.Chat.ID
			isCallback = true
		} else if update.Message != nil {
			userID = update.Message.From.ID
			chatID = update.Message.Chat.ID
		} else {
			continue
		}

		userData, err := b.db.GetUserData(userID)
		if err != nil {
			log.Printf("FATAL: Could not get or create user data for user %d: %v", userID, err)
			b.sendErrorMessage(chatID, "database_error")
			continue
		}

		if isCallback {
			b.handleCallbackQuery(update.CallbackQuery, userData)
			continue
		}

		if update.Message != nil {
			log.Printf("Received message from [ID: %d] with state [%s]", userID, userData.State)
			if update.Message.IsCommand() {
				b.handleCommand(update.Message, userData)
				continue
			}

			switch userData.State {
			case models.StateWaitingForVideo:
				b.handleVideoUpload(update.Message, userData)
			case models.StateWaitingForCustomStyle:
				b.handleCustomStyleInput(update.Message, userData)
			case models.StateWaitingForRevision:
				b.handleRevisionInput(update.Message, userData)
			}
		}
	}
}

func (b *Bot) getFileBytes(fileID string) ([]byte, error) {
	fileURL, err := b.api.GetFileDirectURL(fileID)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (b *Bot) sendErrorMessage(chatID int64, messageID string) {
	text, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID})
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}
