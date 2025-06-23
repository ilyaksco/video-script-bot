package apikeys

import (
	"errors"
	"log"
	"sync"
)

var ErrNoKeysAvailable = errors.New("no API keys available")
var ErrAllKeysExhausted = errors.New("all available API keys have been exhausted")

// KeyManager handles the rotation of API keys.
type KeyManager struct {
	keys         []string
	currentIndex int
	mutex        sync.Mutex
}

// NewManager creates a new KeyManager.
func NewManager(keys []string) (*KeyManager, error) {
	if len(keys) == 0 || (len(keys) == 1 && keys[0] == "") {
		return nil, ErrNoKeysAvailable
	}
	return &KeyManager{
		keys:         keys,
		currentIndex: 0,
	}, nil
}

// GetCurrentKey returns the currently active API key.
func (km *KeyManager) GetCurrentKey() string {
	km.mutex.Lock()
	defer km.mutex.Unlock()
	return km.keys[km.currentIndex]
}

// RotateKey moves to the next available API key.
// It returns ErrAllKeysExhausted if it has looped through all available keys.
func (km *KeyManager) RotateKey() error {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	log.Printf("API key %d has failed or is exhausted. Rotating to the next key.", km.currentIndex+1)
	km.currentIndex++

	// If we've gone past the last key, we've exhausted all options.
	if km.currentIndex >= len(km.keys) {
		log.Println("WARNING: All API keys have been tried and failed.")
		km.currentIndex = 0 // Reset to the first key for the next cycle of requests
		return ErrAllKeysExhausted
	}

	log.Printf("Switched to API key %d.", km.currentIndex+1)
	return nil
}

// GetAllKeys is used for the retry logic to know how many keys to try.
func (km *KeyManager) GetAllKeys() []string {
	return km.keys
}
