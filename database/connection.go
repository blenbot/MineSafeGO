package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

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
		// Add columns if they don't exist (for existing databases)
		`DO $$ BEGIN
			ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture_url TEXT;
			ALTER TABLE users ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]';
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]';
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS likes_count INTEGER DEFAULT 0;
			ALTER TABLE video_modules ADD COLUMN IF NOT EXISTS dislikes_count INTEGER DEFAULT 0;
		EXCEPTION WHEN OTHERS THEN NULL;
		END $$`,
	}

	for _, migration := range migrations {
		if _, err := DB.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, migration)
		}
	}

	log.Println("Migrations completed successfully")

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

	// Videos from database/assets with tags from video.json
	videos := []struct {
		title       string
		videoURL    string
		duration    int
		category    string
		description string
		tags        []string
	}{
		{
			"Machine Safety First",
			"/assets/Machine Safety First.mp4",
			300,
			"Safety",
			"Worker testimonial about machine safety following DGMS guidelines",
			[]string{"worker testimonial", "dgms guidelines"},
		},
		{
			"Pre-Shift check",
			"/assets/Pre-Shift check.mp4",
			240,
			"Safety",
			"Pre-shift safety check procedures as per DGMS guidelines",
			[]string{"pre-shift", "dgms guidelines"},
		},
		{
			"Accident case study",
			"/assets/Accident case study.mp4",
			360,
			"Safety",
			"Case study of gas leak accident and lessons learned",
			[]string{"gas leak", "accident"},
		},
	}

	log.Println("Seeding default video modules from assets...")
	for _, v := range videos {
		tagsJSON, _ := json.Marshal(v.tags)
		var videoID int
		err := DB.QueryRow(
			`INSERT INTO video_modules (title, description, video_url, duration, category, tags, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())
			 RETURNING id`,
			v.title, v.description, v.videoURL, v.duration, v.category, tagsJSON,
		).Scan(&videoID)
		if err != nil {
			return fmt.Errorf("failed to seed video '%s': %w", v.title, err)
		}
		log.Printf("Seeded video: %s (ID: %d)", v.title, videoID)
	}

	log.Printf("Successfully seeded %d default video modules", len(videos))

	// Seed quizzes for each video
	if err := seedDefaultQuizzes(); err != nil {
		log.Printf("Warning: Failed to seed default quizzes: %v", err)
	}

	return nil
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

	// Define quizzes for each video
	quizzes := []struct {
		videoTitle string
		quizTitle  string
		tags       []string
		questions  []struct {
			question string
			options  []string
			correct  int
			tags     []string
		}
	}{
		{
			videoTitle: "Machine Safety First",
			quizTitle:  "Machine Safety Quiz",
			tags:       []string{"worker testimonial", "dgms guidelines", "machine safety"},
			questions: []struct {
				question string
				options  []string
				correct  int
				tags     []string
			}{
				{
					question: "What should you do before operating any machine?",
					options:  []string{"Start immediately", "Check machine condition and safety guards", "Wait for supervisor", "Skip inspection if in hurry"},
					correct:  1,
					tags:     []string{"machine safety", "pre-operation"},
				},
				{
					question: "According to DGMS guidelines, who is responsible for machine safety?",
					options:  []string{"Only supervisor", "Only operator", "Both operator and supervisor", "Safety officer only"},
					correct:  2,
					tags:     []string{"dgms guidelines", "responsibility"},
				},
				{
					question: "What is the first action when you notice a machine malfunction?",
					options:  []string{"Continue working", "Stop the machine and report", "Try to fix it yourself", "Ignore if minor"},
					correct:  1,
					tags:     []string{"emergency", "malfunction"},
				},
			},
		},
		{
			videoTitle: "Pre-Shift check",
			quizTitle:  "Pre-Shift Safety Check Quiz",
			tags:       []string{"pre-shift", "dgms guidelines", "inspection"},
			questions: []struct {
				question string
				options  []string
				correct  int
				tags     []string
			}{
				{
					question: "When should pre-shift checks be performed?",
					options:  []string{"Once a week", "Before every shift", "Only on Monday", "When supervisor asks"},
					correct:  1,
					tags:     []string{"pre-shift", "timing"},
				},
				{
					question: "What should be checked during pre-shift inspection?",
					options:  []string{"Only equipment", "Equipment, ventilation, and safety devices", "Nothing specific", "Only if problems reported"},
					correct:  1,
					tags:     []string{"inspection", "checklist"},
				},
				{
					question: "Who should sign off on pre-shift checks according to DGMS?",
					options:  []string{"Anyone available", "Designated competent person", "New trainee", "No signature needed"},
					correct:  1,
					tags:     []string{"dgms guidelines", "documentation"},
				},
			},
		},
		{
			videoTitle: "Accident case study",
			quizTitle:  "Gas Leak Accident Prevention Quiz",
			tags:       []string{"gas leak", "accident", "prevention"},
			questions: []struct {
				question string
				options  []string
				correct  int
				tags     []string
			}{
				{
					question: "What is the primary cause of gas leak accidents in mines?",
					options:  []string{"Poor ventilation and lack of monitoring", "Too many workers", "Weather conditions", "Equipment color"},
					correct:  0,
					tags:     []string{"gas leak", "causes"},
				},
				{
					question: "What should you do immediately if you detect a gas leak?",
					options:  []string{"Continue working", "Evacuate and alert others", "Try to fix it alone", "Wait and observe"},
					correct:  1,
					tags:     []string{"emergency", "response"},
				},
				{
					question: "How often should gas detection equipment be calibrated?",
					options:  []string{"Never", "Once a year", "As per manufacturer guidelines and DGMS regulations", "Only when broken"},
					correct:  2,
					tags:     []string{"equipment", "maintenance"},
				},
			},
		},
	}

	log.Println("Seeding default quizzes...")
	for _, quiz := range quizzes {
		// Get video ID by title
		var videoID int
		err := DB.QueryRow("SELECT id FROM video_modules WHERE title = $1", quiz.videoTitle).Scan(&videoID)
		if err != nil {
			log.Printf("Warning: Could not find video '%s', skipping quiz", quiz.videoTitle)
			continue
		}

		// Insert quiz
		quizTagsJSON, _ := json.Marshal(quiz.tags)
		var quizID int
		err = DB.QueryRow(`
			INSERT INTO quizzes (video_id, title, tags, created_at, updated_at)
			VALUES ($1, $2, $3, NOW(), NOW())
			RETURNING id
		`, videoID, quiz.quizTitle, quizTagsJSON).Scan(&quizID)
		if err != nil {
			return fmt.Errorf("failed to seed quiz '%s': %w", quiz.quizTitle, err)
		}

		// Insert questions
		for _, q := range quiz.questions {
			optionsJSON, _ := json.Marshal(q.options)
			qTagsJSON, _ := json.Marshal(q.tags)
			_, err = DB.Exec(`
				INSERT INTO quiz_questions (quiz_id, question, options, correct_answer, tags, created_at)
				VALUES ($1, $2, $3, $4, $5, NOW())
			`, quizID, q.question, optionsJSON, q.correct, qTagsJSON)
			if err != nil {
				return fmt.Errorf("failed to seed question for quiz '%s': %w", quiz.quizTitle, err)
			}
		}

		log.Printf("Seeded quiz: %s with %d questions", quiz.quizTitle, len(quiz.questions))
	}

	log.Printf("Successfully seeded %d default quizzes", len(quizzes))
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
