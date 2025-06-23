package models

type UserState string

const (
	StateIdle                    UserState = "idle"
	StateWaitingForVideo         UserState = "waiting_for_video"
	StateWaitingForStyle         UserState = "waiting_for_style"
	StateWaitingForCustomStyle   UserState = "waiting_for_custom_style"
	StateWaitingForRevision      UserState = "waiting_for_revision"
	StateWaitingForVoiceSelection UserState = "waiting_for_voice_selection"
)

type UserData struct {
	State           UserState
	VideoFileID     string
	VideoMimeType   string
	ScriptStyle     string
	GeneratedScript string
}

// NewDefaultUserData creates a user with initial idle state.
func NewDefaultUserData() *UserData {
	return &UserData{
		State: StateIdle,
	}
}

type Voice struct {
	VoiceID string `json:"voice_id"`
	Name    string `json:"name"`
}

type VoicesFile struct {
	Voices []Voice `json:"voices"`
}
