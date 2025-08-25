package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"os"
	"time"
	"video-script-bot/internal/apikeys"
	"video-script-bot/internal/models"
	"video-script-bot/internal/proxy"
)

const elevenLabsAPIURL = "https://api.elevenlabs.io/v1"

type ElevenLabsService struct {
	keyManager *apikeys.KeyManager
	proxyManager  *proxy.Manager
	modelID    string
	httpClient *http.Client
	voices     []models.Voice
	hasProxy      bool
}

func NewElevenLabsService(keyManager *apikeys.KeyManager, modelID string, proxyURL string) (*ElevenLabsService, error) {
	transport := &http.Transport{} // <<< KODE BARU DIMULAI

	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxy)
		log.Printf("ElevenLabs service is configured to use proxy: %s", proxyURL)
	} // <<< KODE BARU BERAKHIR


	service := &ElevenLabsService{
		keyManager: keyManager,
		modelID:    modelID,
		httpClient: &http.Client{
			Transport: transport, // <<< PERUBAHAN DI SINI
			Timeout:   time.Minute * 2,
		},
	}
	if err := service.loadVoicesFromFile("voices.json"); err != nil {
		return nil, fmt.Errorf("failed to load voices from file: %w", err)
	}
	return service, nil

}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return strings.Contains(err.Error(), "proxyconnect") || strings.Contains(err.Error(), "EOF")
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

func (s *ElevenLabsService) TextToSpeech(voiceID, text string, stability, clarity, speed float32) ([]byte, error) {
	apiURL := fmt.Sprintf("%s/text-to-speech/%s", elevenLabsAPIURL, voiceID)
	payload := map[string]interface{}{
		"text":     text,
		"model_id": s.modelID,
		"voice_settings": map[string]float32{
			"stability":        stability,
			"similarity_boost": clarity,
			"style":            0.5, // Nilai default yang disarankan
			"use_speaker_boost": 1,
		},
		"pronunciation_dictionary_locators": []map[string]interface{}{},
		"seed":                nil,
		"previous_text":       nil,
		"next_text":           nil,
		"previous_request_ids":[]string{},
		"next_request_ids":  []string{},
	}
	jsonPayload, _ := json.Marshal(payload)

	maxKeyRetries := len(s.keyManager.GetAllKeys())
	for keyAttempt := 0; keyAttempt < maxKeyRetries; keyAttempt++ {
		
		maxProxyRetries := 1
		if s.hasProxy {
			maxProxyRetries = s.proxyManager.GetTotalProxies()
		}

		// Loop untuk mencoba setiap proxy
		for proxyAttempt := 0; proxyAttempt < maxProxyRetries; proxyAttempt++ {
			transport := &http.Transport{}
			if s.hasProxy {
				currentProxy := s.proxyManager.GetCurrentProxy()
				if currentProxy != nil {
					transport.Proxy = http.ProxyURL(currentProxy)
				}
			}
			s.httpClient.Transport = transport

			req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
			req.Header.Set("xi-api-key", s.keyManager.GetCurrentKey())
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "audio/mpeg")

			resp, err := s.httpClient.Do(req)

			// Jika error jaringan, ganti proxy dan coba lagi
			if isNetworkError(err) {
				log.Printf("Network/Proxy error during ElevenLabs request: %v", err)
				if s.hasProxy {
					s.proxyManager.RotateProxy()
					continue 
				}
				time.Sleep(1 * time.Second)
				continue
			}
			
			if err != nil {
				log.Printf("Unhandled error during ElevenLabs request: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// Jika error kuota, ganti API key dan coba lagi dari proxy pertama
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusUnauthorized {
				log.Printf("Quota/Auth error with ElevenLabs key (Status: %s). Rotating key.", resp.Status)
				resp.Body.Close()
				s.keyManager.RotateKey()
				break // Keluar dari loop proxy, masuk ke loop API key berikutnya
			}

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("ElevenLabs returned non-200 status: %s - %s", resp.Status, string(body))
			}

			// Jika berhasil, kembalikan hasil
			audioBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read audio response body: %w", err)
			}
			return audioBytes, nil
		}
	}

	return nil, fmt.Errorf("all ElevenLabs API keys and proxies failed or were exhausted")
}
