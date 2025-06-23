package storage

import (
	"database/sql"
	"fmt"
	"log"
	"video-script-bot/internal/models"

	_ "github.com/mattn/go-sqlite3"
)

type Storage struct {
	db *sql.DB
}

func New(databasePath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &Storage{db: db}
	if err := storage.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return storage, nil
}

func (s *Storage) initDB() error {
	query := `
    CREATE TABLE IF NOT EXISTS users (
        user_id INTEGER PRIMARY KEY,
        state TEXT NOT NULL,
        video_file_id TEXT,
        video_mime_type TEXT,
        script_style TEXT,
        generated_script TEXT
    );`
	_, err := s.db.Exec(query)
	return err
}

func (s *Storage) GetUserData(userID int64) (*models.UserData, error) {
	var userData models.UserData
	query := `SELECT state, video_file_id, video_mime_type, script_style, generated_script FROM users WHERE user_id = ?`

	// Use pointers to sql.NullString for nullable text fields
	var videoFileID, videoMimeType, scriptStyle, generatedScript sql.NullString

	err := s.db.QueryRow(query, userID).Scan(
		&userData.State,
		&videoFileID,
		&videoMimeType,
		&scriptStyle,
		&generatedScript,
	)

	if err == sql.ErrNoRows {
		// User does not exist, create a new one with default values
		log.Printf("User %d not found in DB, creating new entry.", userID)
		defaultUserData := models.NewDefaultUserData()
		if err := s.SetUserData(userID, defaultUserData); err != nil {
			return nil, err
		}
		return defaultUserData, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user data for user %d: %w", userID, err)
	}

	// Convert sql.NullString to string
	userData.VideoFileID = videoFileID.String
	userData.VideoMimeType = videoMimeType.String
	userData.ScriptStyle = scriptStyle.String
	userData.GeneratedScript = generatedScript.String

	return &userData, nil
}

func (s *Storage) SetUserData(userID int64, data *models.UserData) error {
	query := `
    INSERT OR REPLACE INTO users (user_id, state, video_file_id, video_mime_type, script_style, generated_script)
    VALUES (?, ?, ?, ?, ?, ?);`

	_, err := s.db.Exec(query,
		userID,
		data.State,
		data.VideoFileID,
		data.VideoMimeType,
		data.ScriptStyle,
		data.GeneratedScript,
	)

	if err != nil {
		return fmt.Errorf("failed to set user data for user %d: %w", userID, err)
	}
	log.Printf("Data for user %d saved to DB. State: %s", userID, data.State)
	return nil
}
