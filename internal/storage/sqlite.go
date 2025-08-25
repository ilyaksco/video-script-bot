package storage

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
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
	if _, err := s.db.Exec(query); err != nil {
		return err
	}

	if !s.columnExists("users", "stability") {
		log.Println("Database migration: adding 'stability' column to 'users' table.")
		_, err := s.db.Exec("ALTER TABLE users ADD COLUMN stability REAL DEFAULT 0.75")
		if err != nil {
			return fmt.Errorf("failed to add stability column: %w", err)
		}
	}
	if !s.columnExists("users", "clarity") {
		log.Println("Database migration: adding 'clarity' column to 'users' table.")
		_, err := s.db.Exec("ALTER TABLE users ADD COLUMN clarity REAL DEFAULT 0.75")
		if err != nil {
			return fmt.Errorf("failed to add clarity column: %w", err)
		}
	}
	if !s.columnExists("users", "speed") {
		log.Println("Database migration: adding 'speed' column to 'users' table.")
		_, err := s.db.Exec("ALTER TABLE users ADD COLUMN speed REAL DEFAULT 1.0")
		if err != nil {
			return fmt.Errorf("failed to add speed column: %w", err)
		}
	}
	return nil
}

func (s *Storage) columnExists(tableName, columnName string) bool {
	// Sanitize table name to prevent SQL injection, although it's internally controlled here.
	cleanTableName := strings.ReplaceAll(tableName, "'", "''")
	query := fmt.Sprintf("PRAGMA table_info('%s')", cleanTableName)

	rows, err := s.db.Query(query)
	if err != nil {
		log.Printf("Could not query table info for %s: %v", tableName, err)
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull int
		var dflt_value interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk); err != nil {
			log.Printf("Could not scan table info row: %v", err)
			continue
		}
		if name == columnName {
			return true
		}
	}
	return false
}

func (s *Storage) GetUserData(userID int64) (*models.UserData, error) {
	var userData models.UserData
	query := `SELECT state, video_file_id, video_mime_type, script_style, generated_script, stability, clarity, speed FROM users WHERE user_id = ?`

	var videoFileID, videoMimeType, scriptStyle, generatedScript sql.NullString
	var stability, clarity, speed sql.NullFloat64

	err := s.db.QueryRow(query, userID).Scan(
		&userData.State,
		&videoFileID,
		&videoMimeType,
		&scriptStyle,
		&generatedScript,
		&stability,
		&clarity,
		&speed,
	)

	if err == sql.ErrNoRows {
		log.Printf("User %d not found in DB, creating new entry.", userID)
		defaultUserData := models.NewDefaultUserData()
		if err := s.SetUserData(userID, defaultUserData); err != nil {
			return nil, err
		}
		return defaultUserData, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user data for user %d: %w", userID, err)
	}

	userData.VideoFileID = videoFileID.String
	userData.VideoMimeType = videoMimeType.String
	userData.ScriptStyle = scriptStyle.String
	userData.GeneratedScript = generatedScript.String
	if stability.Valid {
		userData.Stability = float32(stability.Float64)
	} else {
		userData.Stability = models.DefaultStability
	}
	if clarity.Valid {
		userData.Clarity = float32(clarity.Float64)
	} else {
		userData.Clarity = models.DefaultClarity
	}
	if speed.Valid {
		userData.Speed = float32(speed.Float64)
	} else {
		userData.Speed = models.DefaultSpeed
	}

	return &userData, nil
}

func (s *Storage) SetUserData(userID int64, data *models.UserData) error {
	query := `
    INSERT OR REPLACE INTO users (user_id, state, video_file_id, video_mime_type, script_style, generated_script, stability, clarity, speed)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err := s.db.Exec(query,
		userID,
		data.State,
		data.VideoFileID,
		data.VideoMimeType,
		data.ScriptStyle,
		data.GeneratedScript,
		data.Stability,
		data.Clarity,
		data.Speed,
	)

	if err != nil {
		return fmt.Errorf("failed to set user data for user %d: %w", userID, err)
	}
	log.Printf("Data for user %d saved to DB. State: %s, Stability: %.2f, Clarity: %.2f, Speed: %.2f", userID, data.State, data.Stability, data.Clarity, data.Speed)
	return nil
}
