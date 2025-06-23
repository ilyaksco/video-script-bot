package ai

import (
	"context"
	"fmt"
	"log"
	"strings"
	"video-script-bot/internal/apikeys"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiService struct {
	keyManager *apikeys.KeyManager
}

func NewGeminiService(keyManager *apikeys.KeyManager) *GeminiService {
	return &GeminiService{
		keyManager: keyManager,
	}
}

func (s *GeminiService) GenerateScriptFromVideo(ctx context.Context, videoData []byte, mimeType string, style string) (string, error) {
	prompt := fmt.Sprintf(
		"Analyze this video and create a concise, scene-by-scene script. The format must be exactly 'HH:MM:SS-HH:MM:SS: description'. The descriptions must be brief and directly correspond to the visual action in that video segment. Do not add information that is not present in the video. The requested style is: '%s'.",
		style,
	)
	videoPart := genai.Blob{MIMEType: mimeType, Data: videoData}

	// Retry logic with key rotation
	for i := 0; i < len(s.keyManager.GetAllKeys()); i++ {
		apiKey := s.keyManager.GetCurrentKey()
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			log.Printf("Failed to create genai client with key %d: %v. Rotating key.", i+1, err)
			s.keyManager.RotateKey()
			continue
		}

		model := client.GenerativeModel("gemini-1.5-flash")
		res, err := model.GenerateContent(ctx, videoPart, genai.Text(prompt))

		if err != nil {
			if isQuotaError(err) {
				log.Printf("Quota error detected with Gemini key %d. Rotating key.", i+1)
				s.keyManager.RotateKey()
				continue // Try again with the next key
			}
			return "", fmt.Errorf("gemini content generation failed with a non-quota error: %w", err)
		}

		return extractText(res)
	}

	return "", fmt.Errorf("all Gemini API keys failed or were exhausted")
}

func (s *GeminiService) ReviseScript(ctx context.Context, originalScript, instructions string) (string, error) {
	// This function can also be updated with the same retry logic if needed
	prompt := fmt.Sprintf(
		"You are a script editor. Below is an original video script. Revise it based on the user's instructions. Maintain the exact 'HH:MM:SS-HH:MM:SS: description' format for every line. Keep the descriptions concise and relevant to the original script's context.\n\nOriginal Script:\n%s\n\nUser Instructions:\n%s",
		originalScript,
		instructions,
	)

	// Simplified: using only the current key for now. Can be expanded with retry logic.
	apiKey := s.keyManager.GetCurrentKey()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create genai client for revision: %w", err)
	}
	
	model := client.GenerativeModel("gemini-1.5-flash")
	res, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini revision failed: %w", err)
	}

	return extractText(res)
}


func isQuotaError(err error) bool {
	errorString := strings.ToLower(err.Error())
	// Common substrings for quota errors
	return strings.Contains(errorString, "429") || strings.Contains(errorString, "quota") || strings.Contains(errorString, "limit exceeded")
}

func extractText(res *genai.GenerateContentResponse) (string, error) {
	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned no content")
	}
	if textPart, ok := res.Candidates[0].Content.Parts[0].(genai.Text); ok {
		return string(textPart), nil
	}
	return "", fmt.Errorf("gemini response did not contain text")
}
