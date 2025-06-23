package state

import (
	"log"
	"sync"
	"video-script-bot/internal/models"
)

type Manager struct {
	userStates map[int64]models.UserData
	mu         sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		userStates: make(map[int64]models.UserData),
	}
}

func (m *Manager) GetUserData(userID int64) models.UserData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.userStates[userID]
	if !ok {
		// Return a default state if user is not found
		return models.UserData{State: models.StateIdle}
	}
	return data
}

func (m *Manager) SetState(userID int64, state models.UserState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	data := m.userStates[userID]
	data.State = state
	m.userStates[userID] = data
	log.Printf("State for user %d set to %s", userID, state)
}

func (m *Manager) SetUserData(userID int64, data models.UserData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userStates[userID] = data
	log.Printf("Data for user %d updated. New state: %s, FileID: %s", userID, data.State, data.VideoFileID)
}
