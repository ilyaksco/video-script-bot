package models

const (
	DefaultStability = 0.75
	DefaultClarity   = 0.75
)

type UserState string

const (
	StateIdle                    UserState = "idle"
	StateWaitingForVideo         UserState = "waiting_for_video"
	StateWaitingForStyle         UserState = "waiting_for_style"
	StateWaitingForCustomStyle   UserState = "waiting_for_custom_style"
	StateWaitingForRevision      UserState = "waiting_for_revision"
	StateWaitingForVoiceSelection UserState = "waiting_for_voice_selection"
	StateWaitingForStability    UserState = "waiting_for_stability"
	StateWaitingForClarity      UserState = "waiting_for_clarity"
)

type UserData struct {
	State           UserState
	VideoFileID     string
	VideoMimeType   string
	ScriptStyle     string
	GeneratedScript string
	Stability		float32
	Clarity			float32
}

// NewDefaultUserData creates a user with initial idle state.
func NewDefaultUserData() *UserData {
	return &UserData{
		State: StateIdle,
		Stability: DefaultStability,
		Clarity:   DefaultClarity,
	}
}

type Voice struct {
	VoiceID string `json:"voice_id"`
	Name    string `json:"name"`
}

type VoicesFile struct {
	Voices []Voice `json:"voices"`
}
