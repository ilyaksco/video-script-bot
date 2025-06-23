package bot

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"video-script-bot/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

const voicesPerPage = 6

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery, userData *models.UserData) {
	ack := tgbotapi.NewCallback(callback.ID, "")
	if _, err := b.api.Request(ack); err != nil {
		log.Printf("Failed to acknowledge callback query: %v", err)
	}

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
		b.promptForVideoUpload(callback.Message.Chat.ID, userData)
	case "custom_style":
		b.promptForCustomStyle(callback.Message.Chat.ID, userData)
	case "agree_script":
		b.handleAgreeScript(callback, userData)
	case "regenerate_script":
		b.handleRegenerateScript(callback.Message.Chat.ID, userData)
	case "revise_script":
		b.handleReviseScript(callback.Message.Chat.ID, userData)
	default:
		log.Printf("Received unknown callback data: %s", callback.Data)
	}
}

func (b *Bot) handleCommand(message *tgbotapi.Message, userData *models.UserData) {
	switch message.Command() {
	case "start":
		// Reset user data by overwriting the struct pointed to by the userData parameter.
		*userData = *models.NewDefaultUserData()
		b.db.SetUserData(message.From.ID, userData)
		b.handleStartCommand(message.Chat.ID)
	case "voice":
		b.handleVoiceCommand(message)
	default:
		log.Printf("Received an unknown command: %s", message.Command())
	}
}

func (b *Bot) handleVoiceCommand(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	args := message.CommandArguments()

	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		usageText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "voice_command_usage"})
		msg := tgbotapi.NewMessage(chatID, usageText)
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
		b.api.Send(msg)
		return
	}

	generatingText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "generating_audio_simple"})
	msg := tgbotapi.NewMessage(chatID, generatingText)
	b.api.Send(msg)

	go func() {
		audioBytes, err := b.elevenlabsService.TextToSpeech(voiceID, textToConvert)
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
		}
	}()
}

func (b *Bot) handleStartCommand(chatID int64) {
	startMessageText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "start_message"})
	createScriptButtonText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "create_script_button"})

	msg := tgbotapi.NewMessage(chatID, startMessageText)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(createScriptButtonText, "create_script"),
		),
	)
	msg.ReplyMarkup = keyboard

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending start message: %v", err)
	}
}

func (b *Bot) promptForVideoUpload(chatID int64, userData *models.UserData) {
	uploadPromptText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "upload_video_prompt"})
	msg := tgbotapi.NewMessage(chatID, uploadPromptText)
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

	go b.generateScript(context.Background(), chatID, userID, userData)
}

func (b *Bot) promptForCustomStyle(chatID int64, userData *models.UserData) {
	promptText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "custom_style_prompt"})
	msg := tgbotapi.NewMessage(chatID, promptText)
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

	go b.generateScript(context.Background(), chatID, userID, userData)
}

func (b *Bot) generateScript(ctx context.Context, chatID int64, userID int64, userData *models.UserData) {
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
		log.Printf("Error generating script from Gemini for user %d: %v", userID, err)
		b.sendErrorMessage(chatID, "analysis_error")
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
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)

	userData.State = models.StateWaitingForVoiceSelection
	b.db.SetUserData(userID, userData)
	
	b.sendPaginatedVoices(chatID, 0)

	editMsg := tgbotapi.NewEditMessageReplyMarkup(chatID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
	b.api.Send(editMsg)
}

func (b *Bot) handleRegenerateScript(chatID int64, userData *models.UserData) {
	generatingText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "generating_script"})
	msg := tgbotapi.NewMessage(chatID, generatingText)
	b.api.Send(msg)
	go b.generateScript(context.Background(), chatID, chatID, userData)
}

func (b *Bot) handleReviseScript(chatID int64, userData *models.UserData) {
	text, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "revise_prompt"})
	msg := tgbotapi.NewMessage(chatID, text)
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
	
	go b.reviseScript(context.Background(), chatID, userID, instructions, userData)
}

func (b *Bot) reviseScript(ctx context.Context, chatID, userID int64, instructions string, userData *models.UserData) {
	if userData.GeneratedScript == "" {
		log.Printf("Error for user %d: no script to revise", userID)
		b.sendErrorMessage(chatID, "analysis_error")
		return
	}

	revisedScript, err := b.geminiService.ReviseScript(ctx, userData.GeneratedScript, instructions)
	if err != nil {
		log.Printf("Error revising script for user %d: %v", userID, err)
		b.sendErrorMessage(chatID, "analysis_error")
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

func (b *Bot) sendPaginatedVoices(chatID int64, page int) {
	voices := b.elevenlabsService.GetVoices()
	if len(voices) == 0 {
		log.Println("Error: no voices loaded from file")
		b.sendErrorMessage(chatID, "audio_generation_error")
		return
	}

	keyboard := b.getVoiceSelectionKeyboard(voices, page)
	msg := tgbotapi.NewMessage(chatID, "Please select a voice:")
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

	keyboard := b.getVoiceSelectionKeyboard(voices, page)
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
	
	go b.generateAndSendAudio(chatID, userID, voiceID, userData)
}

func (b *Bot) generateAndSendAudio(chatID, userID int64, voiceID string, userData *models.UserData) {
	if userData.GeneratedScript == "" {
		log.Printf("User %d has no script to generate audio from", userID)
		return
	}

	re := regexp.MustCompile(`\r?\n`)
	lines := re.Split(userData.GeneratedScript, -1)

	for _, line := range lines {
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

		audioBytes, err := b.elevenlabsService.TextToSpeech(voiceID, textToSpeak)
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

	completionText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "audio_generation_complete"})
	finalMsg := tgbotapi.NewMessage(chatID, completionText)
	b.api.Send(finalMsg)
}

func (b *Bot) getStyleSelectionKeyboard() tgbotapi.InlineKeyboardMarkup {
	profText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "style_professional"})
	narrText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "style_narrative"})
	custText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "style_custom"})

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(profText, "style_professional"),
			tgbotapi.NewInlineKeyboardButtonData(narrText, "style_narrative"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(custText, "custom_style"),
		),
	)
}

func (b *Bot) getScriptActionKeyboard() tgbotapi.InlineKeyboardMarkup {
	agreeText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_agree"})
	regenText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_regenerate"})
	reviseText, _ := b.localizer.Localize(&i18n.LocalizeConfig{MessageID: "button_revise"})

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(agreeText, "agree_script"),
			tgbotapi.NewInlineKeyboardButtonData(regenText, "regenerate_script"),
			tgbotapi.NewInlineKeyboardButtonData(reviseText, "revise_script"),
		),
	)
}

func (b *Bot) getVoiceSelectionKeyboard(voices []models.Voice, page int) tgbotapi.InlineKeyboardMarkup {
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
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(buttonText, "voice_"+voices[i].VoiceID))

		if i+1 < end {
			buttonText2 := voices[i+1].Name
			if buttonText2 == "" {
				buttonText2 = voices[i+1].VoiceID
			}
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(buttonText2, "voice_"+voices[i+1].VoiceID))
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

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}
