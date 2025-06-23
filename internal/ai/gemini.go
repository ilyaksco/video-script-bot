package ai

import (
	"context"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiService struct {
	client *genai.GenerativeModel
}

func NewGeminiService(ctx context.Context, apiKey string) (*GeminiService, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("could not create new genai client: %w", err)
	}

	model := client.GenerativeModel("gemini-2.5-flash")

	return &GeminiService{
		client: model,
	}, nil
}

func (s *GeminiService) GenerateScriptFromVideo(ctx context.Context, videoData []byte, mimeType string, style string) (string, error) {
	log.Printf("Generating script with style: %s", style)

	prompt := fmt.Sprintf(
		"Analyze this video and create a concise, scene-by-scene script. The format must be exactly 'HH:MM:SS-HH:MM:SS: description'. The descriptions must be brief and directly correspond to the visual action in that video segment. Do not add information that is not present in the video. The requested style is: '%s'.",
		style,
	)

	videoPart := genai.Blob{MIMEType: mimeType, Data: videoData}

	res, err := s.client.GenerateContent(ctx, videoPart, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini content generation failed: %w", err)
	}

	return extractText(res)
}

func (s *GeminiService) ReviseScript(ctx context.Context, originalScript, instructions string) (string, error) {
	log.Printf("Revising script with instructions: %s", instructions)

	prompt := fmt.Sprintf(
		"You are a script editor. Below is an original video script. Revise it based on the user's instructions. Maintain the exact 'HH:MM:SS-HH:MM:SS: description' format for every line. Keep the descriptions concise and relevant to the original script's context.\n\nOriginal Script:\n%s\n\nUser Instructions:\n%s",
		originalScript,
		instructions,
	)

	res, err := s.client.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini revision failed: %w", err)
	}

	return extractText(res)
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
