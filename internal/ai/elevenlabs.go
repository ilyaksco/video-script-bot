package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"video-script-bot/internal/apikeys"
	"video-script-bot/internal/models"
)

const elevenLabsAPIURL = "https://api.elevenlabs.io/v1"

type ElevenLabsService struct {
	keyManager *apikeys.KeyManager
	modelID    string
	httpClient *http.Client
	voices     []models.Voice
}

func NewElevenLabsService(keyManager *apikeys.KeyManager, modelID string) (*ElevenLabsService, error) {
	service := &ElevenLabsService{
		keyManager: keyManager,
		modelID:    modelID,
		httpClient: &http.Client{
			Timeout: time.Minute * 2,
		},
	}
	if err := service.loadVoicesFromFile("voices.json"); err != nil {
		return nil, fmt.Errorf("failed to load voices from file: %w", err)
	}
	return service, nil
}

func (s *ElevenLabsService) loadVoicesFromFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	var voicesFile models.VoicesFile
	if err := json.Unmarshal(bytes, &voicesFile); err != nil {
		return err
	}
	s.voices = voicesFile.Voices
	return nil
}

func (s *ElevenLabsService) GetVoices() []models.Voice {
	return s.voices
}

func (s *ElevenLabsService) TextToSpeech(voiceID, text string, stability, clarity float32) ([]byte, error) {
	url := fmt.Sprintf("%s/text-to-speech/%s", elevenLabsAPIURL, voiceID)
	payload := map[string]interface{}{
		"text":     text,
		"model_id": s.modelID,
		"voice_settings": map[string]float32{
			"stability":        stability,
			"similarity_boost": clarity,
		},
	}
	jsonPayload, _ := json.Marshal(payload)

	for i := 0; i < len(s.keyManager.GetAllKeys()); i++ {
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
		req.Header.Set("xi-api-key", s.keyManager.GetCurrentKey())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "audio/mpeg")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			log.Printf("ElevenLabs request failed (attempt %d): %v", i+1, err)
			time.Sleep(1 * time.Second)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusUnauthorized {
			log.Printf("Quota/Auth error detected with ElevenLabs key %d (Status: %s). Rotating key.", i+1, resp.Status)
			resp.Body.Close()
			s.keyManager.RotateKey()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("ElevenLabs returned non-200 status: %s - %s", resp.Status, string(body))
		}

		audioBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read audio response body: %w", err)
		}
		return audioBytes, nil
	}

	return nil, fmt.Errorf("all ElevenLabs API keys failed or were exhausted")
}
