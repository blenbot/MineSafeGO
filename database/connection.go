package database

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Embed the videos.json file directly into the binary
// This ensures it's always available regardless of working directory (critical for Render deployment)
//
//go:embed videos.json
var embeddedVideosJSON []byte

var DB *sql.DB

func InitDB() error {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	var psqlInfo string

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		psqlInfo = databaseURL
	} else {
		host := os.Getenv("DB_HOST")
		portstr := os.Getenv("DB_PORT")
		port, err := strconv.Atoi(portstr)
		if err != nil {
			return fmt.Errorf("invalid DB_PORT, must be a number: %w", err)
		}
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")
		sslmode := os.Getenv("DB_SSLMODE")

		if sslmode == "" {
			sslmode = "disable"
		}

		psqlInfo = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode)
	}

	var err error
	DB, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		return fmt.Errorf("error opening database: %w", err)
	}

	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(5)

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}

	log.Println("Successfully connected to database")

	if err := runMigrations(); err != nil {
		return fmt.Errorf("error running migrations: %w", err)
	}

	return nil
}

func CloseDB() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

func runMigrations() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			phone VARCHAR(50),
			password VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL,
			mining_site VARCHAR(255),
			location VARCHAR(255),
			supervisor_id VARCHAR(255),
			profile_picture_url TEXT,
			tags JSONB DEFAULT '[]',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS video_modules (
			id SERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			video_url TEXT NOT NULL,
			duration INTEGER,
			category VARCHAR(100),
			thumbnail TEXT,
			tags JSONB DEFAULT '[]',
			likes_count INTEGER DEFAULT 0,
			dislikes_count INTEGER DEFAULT 0,
			is_active BOOLEAN DEFAULT true,
			created_by VARCHAR(255) REFERENCES users(user_id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Video likes/dislikes tracking
		`CREATE TABLE IF NOT EXISTS video_reactions (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(user_id) ON DELETE CASCADE,
			video_id INTEGER REFERENCES video_modules(id) ON DELETE CASCADE,
			reaction_type VARCHAR(10) NOT NULL CHECK (reaction_type IN ('like', 'dislike')),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, video_id)
		)`,
		// Quizzes table (linked to video modules by title or video_id)
		`CREATE TABLE IF NOT EXISTS quizzes (
			id SERIAL PRIMARY KEY,
			video_id INTEGER REFERENCES video_modules(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			tags JSONB DEFAULT '[]',
			created_by VARCHAR(255) REFERENCES users(user_id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Quiz questions (separate from video questions for more flexibility)
		`CREATE TABLE IF NOT EXISTS quiz_questions (
			id SERIAL PRIMARY KEY,
			quiz_id INTEGER REFERENCES quizzes(id) ON DELETE CASCADE,
			question TEXT NOT NULL,
			options JSONB NOT NULL,
			correct_answer INTEGER NOT NULL,
			tags JSONB DEFAULT '[]',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Quiz completions
		`CREATE TABLE IF NOT EXISTS quiz_completions (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(user_id) ON DELETE CASCADE,
			quiz_id INTEGER REFERENCES quizzes(id) ON DELETE CASCADE,
			score INTEGER NOT NULL,
			total_questions INTEGER NOT NULL,
			completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS star_videos (
			id SERIAL PRIMARY KEY,
			video_id INTEGER REFERENCES video_modules(id) ON DELETE CASCADE,
			supervisor_id VARCHAR(255) REFERENCES users(user_id),
			set_date DATE NOT NULL,
			is_active BOOLEAN DEFAULT true,
			UNIQUE(supervisor_id, set_date, is_active)
		)`,
		`CREATE TABLE IF NOT EXISTS questions (
			id SERIAL PRIMARY KEY,
			video_id INTEGER REFERENCES video_modules(id) ON DELETE CASCADE,
			question TEXT NOT NULL,
			options JSONB NOT NULL,
			answer INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS module_completions (
			id SERIAL PRIMARY KEY,
			miner_id VARCHAR(255) REFERENCES users(user_id),
			video_id INTEGER REFERENCES video_modules(id) ON DELETE CASCADE,
			completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			score INTEGER,
			total_questions INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS emergencies (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(user_id),
			emergency_id INTEGER NOT NULL,
			severity VARCHAR(50),
			latitude DOUBLE PRECISION,
			longitude DOUBLE PRECISION,
			issue TEXT,
			media_status VARCHAR(50) DEFAULT 'NOT_APPLICABLE',
			media_url TEXT,
			location TEXT,
			incident_time TIMESTAMP,
			reporting_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			status VARCHAR(50) DEFAULT 'PENDING',
			resolution_time TIMESTAMP,
			UNIQUE(user_id, emergency_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE INDEX IF NOT EXISTS idx_users_user_id ON users(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_supervisor ON users(supervisor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_emergencies_user ON emergencies(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_emergencies_status ON emergencies(status)`,
		`CREATE INDEX IF NOT EXISTS idx_module_completions_miner ON module_completions(miner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_star_videos_active ON star_videos(is_active, set_date)`,
		`CREATE INDEX IF NOT EXISTS idx_video_reactions_user ON video_reactions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_video_reactions_video ON video_reactions(video_id)`,
		`CREATE INDEX IF NOT EXISTS idx_quizzes_video ON quizzes(video_id)`,
		`CREATE INDEX IF NOT EXISTS idx_quiz_completions_user ON quiz_completions(user_id)`,
		// Pre-Start Checklist table
		`CREATE TABLE IF NOT EXISTS pre_start_checklist (
			id SERIAL PRIMARY KEY,
			supervisor_id VARCHAR(255) NOT NULL,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			is_default BOOLEAN DEFAULT false,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// PPE Checklist table
		`CREATE TABLE IF NOT EXISTS ppe_checklist (
			id SERIAL PRIMARY KEY,
			supervisor_id VARCHAR(255) NOT NULL,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			is_default BOOLEAN DEFAULT false,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Pre-Start Checklist Completions (tracks user tick/no tick)
		`CREATE TABLE IF NOT EXISTS pre_start_checklist_completions (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(user_id),
			item_id INTEGER REFERENCES pre_start_checklist(id) ON DELETE CASCADE,
			is_completed BOOLEAN DEFAULT false,
			completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			date DATE NOT NULL,
			UNIQUE(user_id, item_id, date)
		)`,
		// PPE Checklist Completions (tracks user tick/no tick)
		`CREATE TABLE IF NOT EXISTS ppe_checklist_completions (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(user_id),
			item_id INTEGER REFERENCES ppe_checklist(id) ON DELETE CASCADE,
			is_completed BOOLEAN DEFAULT false,
			completed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			date DATE NOT NULL,
			UNIQUE(user_id, item_id, date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pre_start_checklist_supervisor ON pre_start_checklist(supervisor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ppe_checklist_supervisor ON ppe_checklist(supervisor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pre_start_completions_user ON pre_start_checklist_completions(user_id, date)`,
		`CREATE INDEX IF NOT EXISTS idx_ppe_completions_user ON ppe_checklist_completions(user_id, date)`,
		// Mine Zones table for supervisor zone allocation
		`CREATE TABLE IF NOT EXISTS mine_zones (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			location VARCHAR(255),
			capacity INTEGER DEFAULT 50,
			mining_site VARCHAR(255),
			is_active BOOLEAN DEFAULT true,
			created_by VARCHAR(255) REFERENCES users(user_id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Emergency forwards tracking table
		`CREATE TABLE IF NOT EXISTS emergency_forwards (
			id SERIAL PRIMARY KEY,
			emergency_id INTEGER REFERENCES emergencies(id) ON DELETE CASCADE,
			forwarded_by VARCHAR(255) REFERENCES users(user_id),
			recipients TEXT,
			message TEXT,
			forwarded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mine_zones_mining_site ON mine_zones(mining_site)`,
		`CREATE INDEX IF NOT EXISTS idx_mine_zones_active ON mine_zones(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_emergency_forwards_emergency ON emergency_forwards(emergency_id)`,
		// PPE Statistics table - stores daily PPE verification from miners
		`CREATE TABLE IF NOT EXISTS ppe_stats (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(255) REFERENCES users(user_id) ON DELETE CASCADE,
			miner_name VARCHAR(255),
			date DATE NOT NULL DEFAULT CURRENT_DATE,
			safety_helmet VARCHAR(10) DEFAULT 'no',
			protective_gloves VARCHAR(10) DEFAULT 'no',
			safety_shoes VARCHAR(10) DEFAULT 'no',
			high_visibility_vest VARCHAR(10) DEFAULT 'no',
			safety_goggles VARCHAR(10) DEFAULT 'no',
			respirator VARCHAR(10) DEFAULT 'no',
			ear_protection VARCHAR(10) DEFAULT 'no',
			face_shield VARCHAR(10) DEFAULT 'no',
			safety_harness VARCHAR(10) DEFAULT 'no',
			knee_pads VARCHAR(10) DEFAULT 'no',
			manual_checklist JSONB DEFAULT '{}',
			ai_verification JSONB DEFAULT '{}',
			photo_captured BOOLEAN DEFAULT false,
			completion_percentage FLOAT DEFAULT 0,
			items_detected INTEGER DEFAULT 0,
			total_items INTEGER DEFAULT 10,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ppe_stats_user ON ppe_stats(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ppe_stats_date ON ppe_stats(date)`,
		`CREATE INDEX IF NOT EXISTS idx_ppe_stats_user_date ON ppe_stats(user_id, date)`,
		// Add columns if they don't exist (for existing databases)
		`DO $$ BEGIN
			ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture_url TEXT;
			ALTER TABLE users ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]';
			ALTER TABLE users ADD COLUMN IF NOT EXISTS zone_id INTEGER;
			ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]';
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS likes_count INTEGER DEFAULT 0;
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS dislikes_count INTEGER DEFAULT 0;
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS approval_status VARCHAR(50) DEFAULT 'approved';
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS reviewed_by VARCHAR(255);
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP;
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS review_feedback TEXT;
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS views_count INTEGER DEFAULT 0;
		EXCEPTION WHEN OTHERS THEN NULL;
		END $$`,
	}

	for _, migration := range migrations {
		if _, err := DB.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, migration)
		}
	}

	log.Println("Migrations completed successfully")

	// Update existing video URLs to use BASE_URL if set
	if err := updateVideoURLsWithBaseURL(); err != nil {
		log.Printf("Warning: Failed to update video URLs: %v", err)
	}

	// Seed default YouTube video tutorials
	if err := seedDefaultVideos(); err != nil {
		log.Printf("Warning: Failed to seed default videos: %v", err)
	}

	// Seed default checklists
	if err := seedDefaultChecklists(); err != nil {
		log.Printf("Warning: Failed to seed default checklists: %v", err)
	}

	return nil
}

func updateVideoURLsWithBaseURL() error {
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		log.Println("BASE_URL not set, skipping video URL update")
		return nil
	}

	// Update video URLs that start with /assets/ to include the base URL
	result, err := DB.Exec(`
		UPDATE video_modules 
		SET video_url = $1 || video_url 
		WHERE video_url LIKE '/assets/%' 
		AND video_url NOT LIKE 'http%'
	`, baseURL)
	if err != nil {
		return fmt.Errorf("failed to update video URLs: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Updated %d video URLs with BASE_URL: %s", rowsAffected, baseURL)
	}

	// Also update uploaded videos that start with /uploads/
	result, err = DB.Exec(`
		UPDATE video_modules 
		SET video_url = $1 || video_url 
		WHERE video_url LIKE '/uploads/%' 
		AND video_url NOT LIKE 'http%'
	`, baseURL)
	if err != nil {
		return fmt.Errorf("failed to update uploaded video URLs: %w", err)
	}

	rowsAffected, _ = result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Updated %d uploaded video URLs with BASE_URL: %s", rowsAffected, baseURL)
	}

	return nil
}

func seedDefaultVideos() error {
	// Check if videos already exist
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM video_modules").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Videos already seeded, skipping...")
		return nil
	}

	// Load videos from JSON configuration file
	videosConfig, err := loadVideosConfig()
	if err != nil {
		log.Printf("Warning: Could not load videos.json, using defaults: %v", err)
		return nil
	}

	// Get base URL from environment for absolute video URLs
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "" // Will use relative paths if BASE_URL not set (for local dev)
	}

	log.Println("Seeding default video modules from videos.json...")
	for _, v := range videosConfig.Videos {
		videoURL := baseURL + "/assets/" + v.Filename
		tagsJSON, _ := json.Marshal(v.Tags)

		var videoID int
		err := DB.QueryRow(
			`INSERT INTO video_modules (title, description, video_url, duration, category, tags, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())
			 RETURNING id`,
			v.Title, v.Description, videoURL, v.Duration, v.Category, tagsJSON,
		).Scan(&videoID)
		if err != nil {
			return fmt.Errorf("failed to seed video '%s': %w", v.Title, err)
		}
		log.Printf("Seeded video: %s (ID: %d, Language: %s)", v.Title, videoID, v.Language)
	}

	log.Printf("Successfully seeded %d default video modules", len(videosConfig.Videos))

	// Seed quizzes for each video
	if err := seedDefaultQuizzes(); err != nil {
		log.Printf("Warning: Failed to seed default quizzes: %v", err)
	}

	return nil
}

// VideoConfig represents the structure of videos.json
type VideoConfig struct {
	Videos []VideoEntry `json:"videos"`
}

type VideoEntry struct {
	Filename    string     `json:"filename"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Duration    int        `json:"duration"`
	Category    string     `json:"category"`
	Language    string     `json:"language"`
	Tags        []string   `json:"tags"`
	Quiz        *QuizEntry `json:"quiz,omitempty"`
}

type QuizEntry struct {
	Title     string          `json:"title"`
	Questions []QuestionEntry `json:"questions"`
}

type QuestionEntry struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Correct  int      `json:"correct"`
	Tags     []string `json:"tags"`
}

// loadVideosConfig loads video configuration from embedded videos.json
func loadVideosConfig() (*VideoConfig, error) {
	// Use embedded videos.json (compiled into binary - works on Render, Docker, anywhere)
	if len(embeddedVideosJSON) == 0 {
		return nil, fmt.Errorf("embedded videos.json is empty - build error")
	}

	var config VideoConfig
	if err := json.Unmarshal(embeddedVideosJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to parse embedded videos.json: %w", err)
	}

	log.Printf("Loaded %d videos from embedded configuration", len(config.Videos))
	return &config, nil
}

func seedDefaultQuizzes() error {
	// Check if quizzes already exist
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM quizzes").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Quizzes already seeded, skipping...")
		return nil
	}

	// Load videos config which contains quiz data
	videosConfig, err := loadVideosConfig()
	if err != nil {
		log.Printf("Warning: Could not load videos.json for quizzes: %v", err)
		return nil
	}

	log.Println("Seeding default quizzes from videos.json...")
	quizCount := 0
	for _, video := range videosConfig.Videos {
		// Skip videos without quiz data
		if video.Quiz == nil || len(video.Quiz.Questions) == 0 {
			log.Printf("Skipping quiz for video '%s' - no quiz data defined", video.Title)
			continue
		}

		// Get video ID by title
		var videoID int
		err := DB.QueryRow("SELECT id FROM video_modules WHERE title = $1", video.Title).Scan(&videoID)
		if err != nil {
			log.Printf("Warning: Could not find video '%s', skipping quiz", video.Title)
			continue
		}

		// Build quiz tags from video tags
		quizTags := append(video.Tags, video.Category, video.Language)
		quizTagsJSON, _ := json.Marshal(quizTags)

		// Insert quiz
		var quizID int
		err = DB.QueryRow(`
			INSERT INTO quizzes (video_id, title, tags, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			RETURNING id
		`, videoID, video.Quiz.Title, quizTagsJSON).Scan(&quizID)
		if err != nil {
			return fmt.Errorf("failed to seed quiz '%s': %w", video.Quiz.Title, err)
		}

		// Insert questions
		for _, q := range video.Quiz.Questions {
			optionsJSON, _ := json.Marshal(q.Options)
			qTagsJSON, _ := json.Marshal(q.Tags)
			_, err = DB.Exec(`
				INSERT INTO quiz_questions (quiz_id, question, options, correct_answer, tags, created_at)
				VALUES ($1, $2, $3, $4, $5, NOW())
			`, quizID, q.Question, optionsJSON, q.Correct, qTagsJSON)
			if err != nil {
				return fmt.Errorf("failed to seed question for quiz '%s': %w", video.Quiz.Title, err)
			}
		}

		log.Printf("Seeded quiz: %s with %d questions (Language: %s)", video.Quiz.Title, len(video.Quiz.Questions), video.Language)
		quizCount++
	}

	log.Printf("Successfully seeded %d default quizzes from videos.json", quizCount)
	return nil
}

func seedDefaultChecklists() error {
	// Seed Pre-Start Checklist defaults
	var preStartCount int
	err := DB.QueryRow("SELECT COUNT(*) FROM pre_start_checklist WHERE is_default = true").Scan(&preStartCount)
	if err != nil {
		return err
	}

	if preStartCount == 0 {
		preStartItems := []struct {
			title       string
			description string
		}{
			{"Vehicle Inspection", "Check all vehicle fluids, lights, brakes, and tires before operation"},
			{"Communication Check", "Verify radio and communication equipment is functioning properly"},
			{"Work Area Assessment", "Inspect work area for hazards, obstacles, and safe access routes"},
		}

		for _, item := range preStartItems {
			_, err := DB.Exec(`
				INSERT INTO pre_start_checklist (supervisor_id, title, description, is_default, is_active, created_at, updated_at)
				VALUES ('SYSTEM', $1, $2, true, true, NOW(), NOW())
			`, item.title, item.description)
			if err != nil {
				return fmt.Errorf("failed to seed pre-start item '%s': %w", item.title, err)
			}
		}
		log.Printf("Successfully seeded %d default pre-start checklist items", len(preStartItems))
	} else {
		log.Println("Pre-start checklist already seeded, skipping...")
	}

	// Seed PPE Checklist defaults
	var ppeCount int
	err = DB.QueryRow("SELECT COUNT(*) FROM ppe_checklist WHERE is_default = true").Scan(&ppeCount)
	if err != nil {
		return err
	}

	if ppeCount == 0 {
		ppeItems := []struct {
			title       string
			description string
		}{
			{"Hard Hat", "Ensure hard hat is worn and in good condition with no cracks or damage"},
			{"Safety Boots", "Steel-toe safety boots must be worn at all times in operational areas"},
			{"High-Visibility Vest", "High-visibility reflective vest must be worn for visibility"},
		}

		for _, item := range ppeItems {
			_, err := DB.Exec(`
				INSERT INTO ppe_checklist (supervisor_id, title, description, is_default, is_active, created_at, updated_at)
				VALUES ('SYSTEM', $1, $2, true, true, NOW(), NOW())
			`, item.title, item.description)
			if err != nil {
				return fmt.Errorf("failed to seed PPE item '%s': %w", item.title, err)
			}
		}
		log.Printf("Successfully seeded %d default PPE checklist items", len(ppeItems))
	} else {
		log.Println("PPE checklist already seeded, skipping...")
	}

	return nil
}

func extractYouTubeID(url string) string {
	// Extract from embed URL: https://www.youtube.com/embed/VIDEO_ID
	if len(url) > 30 && url[:30] == "https://www.youtube.com/embed/" {
		return url[30:]
	}
	return url
}
