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

	for i := 0; i < len(s.keyManager.GetAllKeys()); i++ {
		if ctx.Err() != nil {
			log.Printf("Context cancelled before attempting API call with key %d.", i+1)
			return "", ctx.Err()
		}

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
			if ctx.Err() == context.Canceled {
				log.Println("Gemini script generation cancelled by user.")
				return "", err
			}
			if isQuotaError(err) {
				log.Printf("Quota error detected with Gemini key %d. Rotating key.", i+1)
				s.keyManager.RotateKey()
				continue
			}
			return "", fmt.Errorf("gemini content generation failed with a non-quota error: %w", err)
		}

		return extractText(res)
	}

	return "", fmt.Errorf("all Gemini API keys failed or were exhausted")
}

func (s *GeminiService) ReviseScript(ctx context.Context, originalScript, instructions string) (string, error) {
	prompt := fmt.Sprintf(
		"You are a script editor. Below is an original video script. Revise it based on the user's instructions. Maintain the exact 'HH:MM:SS-HH:MM:SS: description' format for every line. Keep the descriptions concise and relevant to the original script's context.\n\nOriginal Script:\n%s\n\nUser Instructions:\n%s",
		originalScript,
		instructions,
	)

	for i := 0; i < len(s.keyManager.GetAllKeys()); i++ {
		if ctx.Err() != nil {
			log.Printf("Context cancelled before attempting API call with key %d.", i+1)
			return "", ctx.Err()
		}
		
		apiKey := s.keyManager.GetCurrentKey()
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			log.Printf("Failed to create genai client with key %d for revision: %v. Rotating key.", i+1, err)
			s.keyManager.RotateKey()
			continue
		}

		model := client.GenerativeModel("gemini-1.5-flash")
		res, err := model.GenerateContent(ctx, genai.Text(prompt))

		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Println("Gemini script revision cancelled by user.")
				return "", err
			}
			if isQuotaError(err) {
				log.Printf("Quota error detected with Gemini key %d during revision. Rotating key.", i+1)
				s.keyManager.RotateKey()
				continue
			}
			return "", fmt.Errorf("gemini revision failed with a non-quota error: %w", err)
		}

		return extractText(res)
	}

	return "", fmt.Errorf("all Gemini API keys failed or were exhausted during revision")
}

func isQuotaError(err error) bool {
	errorString := strings.ToLower(err.Error())
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
