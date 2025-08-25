package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"video-script-bot/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const voicesPerPage = 6

func (b *Bot) handleInlineQuery(inlineQuery *tgbotapi.InlineQuery) {
	query := inlineQuery.Query
	userID := inlineQuery.From.ID

	parts := strings.SplitN(query, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		answer := tgbotapi.InlineConfig{
			InlineQueryID:     inlineQuery.ID,
			Results:           []interface{}{},
			IsPersonal:        true,
			SwitchPMText:      b.localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "inline_guide_button"}),
			SwitchPMParameter: "inline_help",
		}
		if _, err := b.api.Request(answer); err != nil {
			log.Printf("Failed to send inline query guide response: %v", err)
		}
		return
	}

	voiceName := parts[0]
	textToConvert := parts[1]

	if len(textToConvert) > 250 {
		return
	}

	userData, err := b.db.GetUserData(userID)
	if err != nil {
		log.Printf("Could not get user data for inline query from user %d: %v. Falling back to defaults.", userID, err)
		userData = models.NewDefaultUserData()
	}

	var results []interface{}
	allVoices := b.elevenlabsService.GetVoices()

	for _, voice := range allVoices {
		if strings.HasPrefix(strings.ToLower(voice.Name), strings.ToLower(voiceName)) {
			audioBytes, err := b.elevenlabsService.TextToSpeech(voice.VoiceID, textToConvert, userData.Stability, userData.Clarity, userData.Speed)			
			if err != nil {
				log.Printf("Inline audio generation failed for voice %s: %v", voice.Name, err)
				continue
			}

			descriptiveFilename := fmt.Sprintf("%s.mp3", textToConvert)
			audioFile := tgbotapi.FileBytes{Name: descriptiveFilename, Bytes: audioBytes}
			audioMsg := tgbotapi.NewAudio(b.cfg.StorageChannelID, audioFile)
			audioMsg.Title = textToConvert
			audioMsg.Performer = voice.Name

			sentMsg, err := b.api.Send(audioMsg)
			if err != nil {
				log.Printf("Failed to send audio to storage channel: %v", err)
				continue
			}

			if sentMsg.Audio == nil {
				log.Printf("Message sent to storage channel does not contain audio")
				continue
			}

			fileID := sentMsg.Audio.FileID
			result := tgbotapi.NewInlineQueryResultCachedAudio(uuid.NewString(), fileID)
			result.Caption = textToConvert

			results = append(results, result)
		}
	}

	answer := tgbotapi.InlineConfig{
		InlineQueryID: inlineQuery.ID,
		Results:       results,
		CacheTime:     1,
		IsPersonal:    true,
	}

	if _, err := b.api.Request(answer); err != nil {
		log.Printf("Failed to send inline query response: %v", err)
	}
}

func (b *Bot) sendInlineHelp(chatID int64) {
	text, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "inline_help_message"})
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	b.api.Send(msg)
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery, userData *models.UserData) {
	ack := tgbotapi.NewCallback(callback.ID, "")

	if strings.HasPrefix(callback.Data, "list_voice_") {
		voiceName := strings.TrimPrefix(callback.Data, "list_voice_")
		if _, err := b.api.Request(ack); err != nil {
			log.Printf("Failed to acknowledge callback query: %v", err)
		}

		replyText, _ := b.localizer.Localize(&i18n.LocalizeConfig{
			MessageID: "voice_command_copied",
			TemplateData: map[string]string{
				"VoiceName": voiceName,
			},
		})

		msg := tgbotapi.NewMessage(callback.Message.Chat.ID, replyText)
		msg.ParseMode = tgbotapi.ModeHTML
		b.api.Send(msg)

		return
	}

	if _, err := b.api.Request(ack); err != nil {
		log.Printf("Failed to acknowledge callback query: %v", err)
	}

	chatID := callback.Message.Chat.ID
	userID := callback.From.ID

	if strings.HasPrefix(callback.Data, "style_") {
		b.handleStyleSelection(callback, userData)
		return
	}
	if strings.HasPrefix(callback.Data, "voice_page_") {
		b.handleVoicePage(callback)
		return
	}
	if strings.HasPrefix(callback.Data, "voice_") {
		b.handleVoiceSelection(callback, userData)
		return
	}

	switch callback.Data {
	case "create_script":
		b.promptForVideoUpload(chatID, userData)
	case "show_voice_tutorial":
		b.sendVoiceTutorial(chatID)
	case "cancel_process":
		b.handleCancelCommand(callback.Message, userData)
	case "custom_style":
		b.promptForCustomStyle(chatID, userData)
	case "agree_script":
		b.handleAgreeScript(callback, userData)
	case "regenerate_script":
		b.handleRegenerateScript(chatID, userData)
	case "revise_script":
		b.handleReviseScript(chatID, userData)
	case "settings":
		b.sendSettingsMenu(chatID, userData, 0)
	case "set_stability":
		promptText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "prompt_stability"})
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = tgbotapi.ModeHTML
		b.api.Send(msg)
		userData.State = models.StateWaitingForStability
		b.db.SetUserData(userID, userData)
	case "set_clarity":
		promptText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "prompt_clarity"})
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = tgbotapi.ModeHTML
		b.api.Send(msg)
		userData.State = models.StateWaitingForClarity
		b.db.SetUserData(userID, userData)
	case "set_speed":
		promptText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "prompt_speed"})
		msg := tgbotapi.NewMessage(chatID, promptText)
		msg.ParseMode = tgbotapi.ModeHTML
		b.api.Send(msg)
		userData.State = models.StateWaitingForSpeed
		b.db.SetUserData(userID, userData)
	case "back_to_main_menu":
		b.handleStartCommand(chatID)
		editMsg := tgbotapi.NewEditMessageReplyMarkup(chatID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
		b.api.Send(editMsg)
	default:
		log.Printf("Received unknown callback data: %s", callback.Data)
	}
}

func (b *Bot) handleCommand(message *tgbotapi.Message, userData *models.UserData) {
	switch message.Command() {
	case "start":
		if strings.Contains(message.CommandArguments(), "inline_help") {
			b.sendInlineHelp(message.Chat.ID)
		} else {
			b.handleStartCommand(message.Chat.ID)
		}
	case "settings":
		b.sendSettingsMenu(message.Chat.ID, userData, 0)
	case "voice":
		b.handleVoiceCommand(message, userData)
	case "listvoices":
		b.handleListVoicesCommand(message.Chat.ID)
	case "help":
		b.handleHelpCommand(message.Chat.ID)
	case "donate": // <-- PENAMBAHAN DI SINI
        b.handleDonateCommand(message.Chat.ID)
	case "cancel":
		b.handleCancelCommand(message, userData)
	default:
		log.Printf("Received an unknown command: %s", message.Command())
	}
}

func (b *Bot) handleListVoicesCommand(chatID int64) {
	b.sendPaginatedVoices(chatID, 0, false)
}

func (b *Bot) handleHelpCommand(chatID int64) {
	helpText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "help_message"})
	msg := tgbotapi.NewMessage(chatID, helpText)
	msg.ParseMode = tgbotapi.ModeHTML
	b.api.Send(msg)
}

func (b *Bot) handleDonateCommand(chatID int64) {
    donateText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "donate_message"})
    msg := tgbotapi.NewMessage(chatID, donateText)
    msg.ParseMode = tgbotapi.ModeHTML
    msg.ReplyMarkup = b.getDonateKeyboard() // Menggunakan keyboard donasi
    b.api.Send(msg)
}

func (b *Bot) handleCancelCommand(message *tgbotapi.Message, userData *models.UserData) {
	userID := message.From.ID
	chatID := message.Chat.ID

	b.cancelBackgroundTask(userID)

	*userData = *models.NewDefaultUserData()
	b.db.SetUserData(userID, userData)

	cancelText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "cancel_message"})
	msg := tgbotapi.NewMessage(chatID, cancelText)
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	b.api.Send(msg)

	b.handleStartCommand(chatID)
}

func (b *Bot) handleVoiceCommand(message *tgbotapi.Message, userData *models.UserData) {
	chatID := message.Chat.ID
	args := message.CommandArguments()

	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		usageText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "voice_command_usage"})
		msg := tgbotapi.NewMessage(chatID, usageText)
		msg.ParseMode = tgbotapi.ModeHTML
		b.api.Send(msg)
		return
	}

	voiceName := strings.ToLower(parts[0])
	textToConvert := parts[1]

	voices := b.elevenlabsService.GetVoices()
	var voiceID string
	var found bool
	for _, voice := range voices {
		name := voice.Name
		if name == "" {
			name = voice.VoiceID
		}
		if strings.ToLower(name) == voiceName {
			voiceID = voice.VoiceID
			found = true
			break
		}
	}

	if !found {
		notFoundText, _ := b.localizer.Localize(&i18n.LocalizeConfig{
			MessageID: "voice_not_found",
			TemplateData: map[string]string{
				"VoiceName": parts[0],
			},
		})
		msg := tgbotapi.NewMessage(chatID, notFoundText)
		msg.ParseMode = tgbotapi.ModeHTML
		b.api.Send(msg)
		return
	}

	generatingText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "generating_audio_simple"})
	msg := tgbotapi.NewMessage(chatID, generatingText)
	b.api.Send(msg)

	go func() {
		audioBytes, err := b.elevenlabsService.TextToSpeech(voiceID, textToConvert, userData.Stability, userData.Clarity, userData.Speed)
		if err != nil {
			log.Printf("Failed to generate direct audio for user %d: %v", message.From.ID, err)
			b.sendErrorMessage(chatID, "audio_generation_error")
			return
		}

		audioFile := tgbotapi.FileBytes{
			Name:  fmt.Sprintf("voice_%d.mp3", message.From.ID),
			Bytes: audioBytes,
		}

		audioMsg := tgbotapi.NewAudio(chatID, audioFile)
		audioMsg.Caption = fmt.Sprintf("Teks: \"%s\"", textToConvert)
		if _, err := b.api.Send(audioMsg); err != nil {
			log.Printf("Failed to send direct audio file for user %d: %v", message.From.ID, err)
			if strings.Contains(err.Error(), "caption is too long") {
				b.sendErrorMessage(chatID, "caption_too_long_error")
			} else {
				b.sendErrorMessage(chatID, "audio_generation_error")
			}
		}
	}()
}

func (b *Bot) handleStartCommand(chatID int64) {
	startMessageText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "start_message"})
	msg := tgbotapi.NewMessage(chatID, startMessageText)
	msg.ReplyMarkup = b.getStartKeyboard()

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending start message: %v", err)
	}
}

func (b *Bot) sendVoiceTutorial(chatID int64) {
	tutorialText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "voice_tutorial_message"})
	msg := tgbotapi.NewMessage(chatID, tutorialText)
	msg.ParseMode = tgbotapi.ModeHTML
	b.api.Send(msg)
}

func (b *Bot) promptForVideoUpload(chatID int64, userData *models.UserData) {
	uploadPromptText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "upload_video_prompt"})
	msg := tgbotapi.NewMessage(chatID, uploadPromptText)

	msg.ReplyMarkup = b.getCancelKeyboard()
	b.api.Send(msg)

	userData.State = models.StateWaitingForVideo
	b.db.SetUserData(chatID, userData)
}

func (b *Bot) handleVideoUpload(message *tgbotapi.Message, userData *models.UserData) {
	chatID := message.Chat.ID

	if message.Video == nil {
		pleaseUploadVideoText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "please_upload_video"})
		msg := tgbotapi.NewMessage(chatID, pleaseUploadVideoText)
		b.api.Send(msg)
		return
	}

	userData.State = models.StateWaitingForStyle
	userData.VideoFileID = message.Video.FileID
	userData.VideoMimeType = message.Video.MimeType
	b.db.SetUserData(message.From.ID, userData)

	chooseStyleText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "choose_script_style"})
	msg := tgbotapi.NewMessage(chatID, chooseStyleText)
	msg.ReplyMarkup = b.getStyleSelectionKeyboard()
	b.api.Send(msg)
}

func (b *Bot) handleStyleSelection(callback *tgbotapi.CallbackQuery, userData *models.UserData) {
	chatID := callback.Message.Chat.ID
	userID := callback.From.ID
	style := strings.TrimPrefix(callback.Data, "style_")

	generatingText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "generating_script"})
	msg := tgbotapi.NewMessage(chatID, generatingText)
	b.api.Send(msg)

	userData.State = models.StateIdle
	userData.ScriptStyle = style
	b.db.SetUserData(userID, userData)

	ctx, _ := b.registerBackgroundTask(userID)
	go b.generateScript(ctx, chatID, userID, userData)
}

func (b *Bot) promptForCustomStyle(chatID int64, userData *models.UserData) {
	promptText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "custom_style_prompt"})
	msg := tgbotapi.NewMessage(chatID, promptText)
	msg.ReplyMarkup = b.getCancelKeyboard()
	b.api.Send(msg)

	userData.State = models.StateWaitingForCustomStyle
	b.db.SetUserData(chatID, userData)
}

func (b *Bot) handleCustomStyleInput(message *tgbotapi.Message, userData *models.UserData) {
	chatID := message.Chat.ID
	userID := message.From.ID
	style := message.Text

	generatingText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "generating_script"})
	msg := tgbotapi.NewMessage(chatID, generatingText)
	b.api.Send(msg)

	userData.State = models.StateIdle
	userData.ScriptStyle = style
	b.db.SetUserData(userID, userData)

	ctx, _ := b.registerBackgroundTask(userID)
	go b.generateScript(ctx, chatID, userID, userData)
}

func (b *Bot) generateScript(ctx context.Context, chatID int64, userID int64, userData *models.UserData) {
	defer b.clearBackgroundTask(userID)

	if userData.VideoFileID == "" || userData.ScriptStyle == "" {
		log.Printf("Error for user %d: missing data for script generation", userID)
		b.sendErrorMessage(chatID, "analysis_error")
		return
	}

	videoBytes, err := b.getFileBytes(userData.VideoFileID)
	if err != nil {
		log.Printf("Error getting file bytes for user %d: %v", userID, err)
		b.sendErrorMessage(chatID, "analysis_error")
		return
	}

	script, err := b.geminiService.GenerateScriptFromVideo(ctx, videoBytes, userData.VideoMimeType, userData.ScriptStyle)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Printf("Script generation cancelled for user %d", userID)
		} else {
			log.Printf("Error generating script from Gemini for user %d: %v", userID, err)
			b.sendErrorMessage(chatID, "analysis_error")
		}
		return
	}

	userData.GeneratedScript = script
	b.db.SetUserData(userID, userData)

	b.sendScriptMessage(chatID, script)
}

func (b *Bot) handleAgreeScript(callback *tgbotapi.CallbackQuery, userData *models.UserData) {
	chatID := callback.Message.Chat.ID
	userID := callback.From.ID

	text, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "agreed_to_script"})
	b.api.Send(tgbotapi.NewMessage(chatID, text))

	userData.State = models.StateWaitingForVoiceSelection
	b.db.SetUserData(userID, userData)

	b.sendPaginatedVoices(chatID, 0, true)

	editMsg := tgbotapi.NewEditMessageReplyMarkup(chatID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
	b.api.Send(editMsg)
}

func (b *Bot) handleRegenerateScript(chatID int64, userData *models.UserData) {
	generatingText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "generating_script"})
	msg := tgbotapi.NewMessage(chatID, generatingText)
	b.api.Send(msg)

	ctx, _ := b.registerBackgroundTask(chatID)
	go b.generateScript(ctx, chatID, chatID, userData)
}

func (b *Bot) handleReviseScript(chatID int64, userData *models.UserData) {
	text, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "revise_prompt"})
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = b.getCancelKeyboard()
	b.api.Send(msg)

	userData.State = models.StateWaitingForRevision
	b.db.SetUserData(chatID, userData)
}

func (b *Bot) handleRevisionInput(message *tgbotapi.Message, userData *models.UserData) {
	chatID := message.Chat.ID
	userID := message.From.ID
	instructions := message.Text

	generatingText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "revision_generating"})
	msg := tgbotapi.NewMessage(chatID, generatingText)
	b.api.Send(msg)

	userData.State = models.StateIdle
	b.db.SetUserData(userID, userData)

	ctx, _ := b.registerBackgroundTask(userID)
	go b.reviseScript(ctx, chatID, userID, instructions, userData)
}

func (b *Bot) reviseScript(ctx context.Context, chatID, userID int64, instructions string, userData *models.UserData) {
	defer b.clearBackgroundTask(userID)
	if userData.GeneratedScript == "" {
		log.Printf("Error for user %d: no script to revise", userID)
		b.sendErrorMessage(chatID, "analysis_error")
		return
	}

	revisedScript, err := b.geminiService.ReviseScript(ctx, userData.GeneratedScript, instructions)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Printf("Script revision cancelled for user %d", userID)
		} else {
			log.Printf("Error revising script for user %d: %v", userID, err)
			b.sendErrorMessage(chatID, "analysis_error")
		}
		return
	}

	userData.GeneratedScript = revisedScript
	b.db.SetUserData(userID, userData)

	b.sendScriptMessage(chatID, revisedScript)
}

func (b *Bot) sendScriptMessage(chatID int64, script string) {
	headerText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "script_generated_header"})
	fullMessage := fmt.Sprintf("<b>%s</b>\n\n<code>%s</code>", headerText, script)
	msg := tgbotapi.NewMessage(chatID, fullMessage)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = b.getScriptActionKeyboard()
	b.api.Send(msg)
}

func (b *Bot) sendPaginatedVoices(chatID int64, page int, forSelection bool) {
	voices := b.elevenlabsService.GetVoices()
	if len(voices) == 0 {
		log.Println("Error: no voices loaded from file")
		b.sendErrorMessage(chatID, "audio_generation_error")
		return
	}

	keyboard := b.getVoiceSelectionKeyboard(voices, page, forSelection)
	msgText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "voice_list_header"})
	if forSelection {
		msgText, _ = b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "agreed_to_script"})
	}

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard
	b.api.Send(msg)
}

func (b *Bot) handleVoicePage(callback *tgbotapi.CallbackQuery) {
	pageStr := strings.TrimPrefix(callback.Data, "voice_page_")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		log.Printf("Invalid page number in callback: %v", err)
		return
	}

	voices := b.elevenlabsService.GetVoices()
	if len(voices) == 0 {
		return
	}

	hasCancel := false
	if callback.Message.ReplyMarkup != nil {
		for _, row := range callback.Message.ReplyMarkup.InlineKeyboard {
			for _, button := range row {
				if button.CallbackData != nil && *button.CallbackData == "cancel_process" {
					hasCancel = true
					break
				}
			}
			if hasCancel {
				break
			}
		}
	}

	keyboard := b.getVoiceSelectionKeyboard(voices, page, hasCancel)
	editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, callback.Message.Text)
	editMsg.ReplyMarkup = &keyboard
	b.api.Send(editMsg)
}

func (b *Bot) handleVoiceSelection(callback *tgbotapi.CallbackQuery, userData *models.UserData) {
	chatID := callback.Message.Chat.ID
	userID := callback.From.ID
	voiceID := strings.TrimPrefix(callback.Data, "voice_")

	if userData.State != models.StateWaitingForVoiceSelection {
		log.Printf("User %d not in voice selection state, ignoring callback.", userID)
		return
	}

	text, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "generating_audio"})
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)

	editMsg := tgbotapi.NewEditMessageReplyMarkup(chatID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
	b.api.Send(editMsg)

	userData.State = models.StateIdle
	b.db.SetUserData(userID, userData)

	ctx, _ := b.registerBackgroundTask(userID)
	go b.generateAndSendAudio(ctx, chatID, userID, voiceID, userData)
}

func (b *Bot) generateAndSendAudio(ctx context.Context, chatID, userID int64, voiceID string, userData *models.UserData) {
	defer b.clearBackgroundTask(userID)
	if userData.GeneratedScript == "" {
		log.Printf("User %d has no script to generate audio from", userID)
		return
	}

	re := regexp.MustCompile(`\r?\n`)
	lines := re.Split(userData.GeneratedScript, -1)

	for _, line := range lines {
		if ctx.Err() != nil {
			log.Printf("Audio generation cancelled for user %d", userID)
			break
		}

		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}

		parts := strings.SplitN(trimmedLine, ": ", 2)
		var textToSpeak string
		if len(parts) > 1 {
			textToSpeak = parts[1]
		} else {
			textToSpeak = parts[0]
		}

		if strings.TrimSpace(textToSpeak) == "" {
			continue
		}

		audioBytes, err := b.elevenlabsService.TextToSpeech(voiceID, textToSpeak, userData.Stability, userData.Clarity, userData.Speed)
		if err != nil {
			log.Printf("Failed to generate audio for line '%s': %v", trimmedLine, err)
			continue
		}

		audioFile := tgbotapi.FileBytes{
			Name:  fmt.Sprintf("audio_%d.mp3", userID),
			Bytes: audioBytes,
		}

		audioMsg := tgbotapi.NewAudio(chatID, audioFile)
		audioMsg.Caption = trimmedLine
		if _, err := b.api.Send(audioMsg); err != nil {
			log.Printf("Failed to send audio file: %v", err)
		}
	}

	if ctx.Err() == nil {
		completionText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "audio_generation_complete"})
		finalMsg := tgbotapi.NewMessage(chatID, completionText)
		b.api.Send(finalMsg)
	}

}

func (b *Bot) sendSettingsMenu(chatID int64, userData *models.UserData, messageID int) {
	text := fmt.Sprintf(
		b.localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "settings_menu_header"}),
		userData.Stability,
		userData.Clarity,
		userData.Speed,
	)

	keyboard := b.getSettingsKeyboard()

	if messageID == 0 {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		b.api.Send(msg)
	} else {
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		editMsg.ParseMode = tgbotapi.ModeHTML
		editMsg.ReplyMarkup = &keyboard
		b.api.Send(editMsg)
	}
}

// --- PENAMBAHAN FUNGSI BARU DIMULAI ---
func (b *Bot) handleSpeedInput(message *tgbotapi.Message, userData *models.UserData) {
	chatID := message.Chat.ID
	userID := message.From.ID

	value, err := strconv.ParseFloat(message.Text, 32)
	if err != nil || value < 0.5 || value > 2.0 {
		errorText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_invalid_speed_value"})
		msg := tgbotapi.NewMessage(chatID, errorText)
		b.api.Send(msg)
		b.sendSettingsMenu(chatID, userData, 0)
		return
	}

	userData.Speed = float32(value)
	userData.State = models.StateIdle
	b.db.SetUserData(userID, userData)

	successText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_updated"})
	b.api.Send(tgbotapi.NewMessage(chatID, successText))
	b.sendSettingsMenu(chatID, userData, 0)
}
// --- PENAMBAHAN FUNGSI BARU SELESAI ---

func (b *Bot) handleStabilityInput(message *tgbotapi.Message, userData *models.UserData) {
	chatID := message.Chat.ID
	userID := message.From.ID

	value, err := strconv.ParseFloat(message.Text, 32)
	if err != nil || value < 0.0 || value > 1.0 {
		errorText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_invalid_value"})
		msg := tgbotapi.NewMessage(chatID, errorText)
		b.api.Send(msg)
		b.sendSettingsMenu(chatID, userData, 0)
		return
	}

	userData.Stability = float32(value)
	userData.State = models.StateIdle
	b.db.SetUserData(userID, userData)

	successText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_updated"})
	b.api.Send(tgbotapi.NewMessage(chatID, successText))
	b.sendSettingsMenu(chatID, userData, 0)
}

func (b *Bot) handleClarityInput(message *tgbotapi.Message, userData *models.UserData) {
	chatID := message.Chat.ID
	userID := message.From.ID

	value, err := strconv.ParseFloat(message.Text, 32)
	if err != nil || value < 0.0 || value > 1.0 {
		errorText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_invalid_value"})
		msg := tgbotapi.NewMessage(chatID, errorText)
		b.api.Send(msg)
		b.sendSettingsMenu(chatID, userData, 0)
		return
	}

	userData.Clarity = float32(value)
	userData.State = models.StateIdle
	b.db.SetUserData(userID, userData)

	successText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_updated"})
	b.api.Send(tgbotapi.NewMessage(chatID, successText))
	b.sendSettingsMenu(chatID, userData, 0)
}

func (b *Bot) getStartKeyboard() tgbotapi.InlineKeyboardMarkup {
	createScriptButtonText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "create_script_button"})
	textToVoiceButtonText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_text_to_voice"})
	settingsButtonText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_button_main"})

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(createScriptButtonText, "create_script"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(textToVoiceButtonText, "show_voice_tutorial"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(settingsButtonText, "settings"),
		),
	)
}

func (b *Bot) getCancelKeyboard() tgbotapi.InlineKeyboardMarkup {
	cancelButtonText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_cancel"})
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(cancelButtonText, "cancel_process"),
		),
	)
}

func (b *Bot) getStyleSelectionKeyboard() tgbotapi.InlineKeyboardMarkup {
	profText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "style_professional"})
	narrText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "style_narrative"})
	custText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "style_custom"})

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(profText, "style_professional"),
			tgbotapi.NewInlineKeyboardButtonData(narrText, "style_narrative"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(custText, "custom_style"),
		),
	)
	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, b.getCancelKeyboard().InlineKeyboard...)
	return keyboard
}

func (b *Bot) getScriptActionKeyboard() tgbotapi.InlineKeyboardMarkup {
	agreeText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_agree"})
	regenText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_regenerate"})
	reviseText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_revise"})
	cancelText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_cancel"})

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(agreeText, "agree_script"),
			tgbotapi.NewInlineKeyboardButtonData(regenText, "regenerate_script"),
			tgbotapi.NewInlineKeyboardButtonData(reviseText, "revise_script"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(cancelText, "cancel_process"),
		),
	)
}

func (b *Bot) getVoiceSelectionKeyboard(voices []models.Voice, page int, forSelection bool) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton

	start := page * voicesPerPage
	end := start + voicesPerPage
	if end > len(voices) {
		end = len(voices)
	}

	for i := start; i < end; i += 2 {
		var row []tgbotapi.InlineKeyboardButton
		buttonText := voices[i].Name
		if buttonText == "" {
			buttonText = voices[i].VoiceID
		}

		var callbackData string
		if forSelection {
			callbackData = "voice_" + voices[i].VoiceID
		} else {
			callbackData = "list_voice_" + buttonText
		}
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData))

		if i+1 < end {
			buttonText2 := voices[i+1].Name
			if buttonText2 == "" {
				buttonText2 = voices[i+1].VoiceID
			}

			var callbackData2 string
			if forSelection {
				callbackData2 = "voice_" + voices[i+1].VoiceID
			} else {
				callbackData2 = "list_voice_" + buttonText2
			}
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(buttonText2, callbackData2))
		}
		rows = append(rows, row)
	}

	var navRow []tgbotapi.InlineKeyboardButton
	if page > 0 {
		prevText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_prev_page"})
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(prevText, fmt.Sprintf("voice_page_%d", page-1)))
	}
	if end < len(voices) {
		nextText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_next_page"})
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData(nextText, fmt.Sprintf("voice_page_%d", page+1)))
	}

	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	if forSelection {
		rows = append(rows, b.getCancelKeyboard().InlineKeyboard...)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (b *Bot) getDonateKeyboard() tgbotapi.InlineKeyboardMarkup {
    var rows [][]tgbotapi.InlineKeyboardButton
    var buttons []tgbotapi.InlineKeyboardButton

    saweriaText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_saweria"})
    bmacText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_buymeacoffee"})

    if b.cfg.SaweriaLink != "" {
        buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL(saweriaText, b.cfg.SaweriaLink))
    }
    if b.cfg.BuyMeACoffeeLink != "" {
        buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL(bmacText, b.cfg.BuyMeACoffeeLink))
    }

    if len(buttons) > 0 {
        rows = append(rows, buttons)
    }

    return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (b *Bot) getSettingsKeyboard() tgbotapi.InlineKeyboardMarkup {
	stabilityBtn, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_button_stability"})
	clarityBtn, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_button_clarity"})
	speedBtn, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_button_speed"})
	backBtn, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "settings_button_back"})

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(stabilityBtn, "set_stability"),
			tgbotapi.NewInlineKeyboardButtonData(clarityBtn, "set_clarity"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(speedBtn, "set_speed"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(backBtn, "back_to_main_menu"),
		),
	)
}
