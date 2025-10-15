package database

import (
	"database/sql"
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
			is_active BOOLEAN DEFAULT true,
			created_by VARCHAR(255) REFERENCES users(user_id),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

	videos := []struct {
		title       string
		videoURL    string
		duration    int
		category    string
		description string
	}{
		{"Mobile Equipment Safety", "https://www.youtube.com/embed/--P3CQY8lYI", 535, "Equipment", "Essential safety guidelines for operating mobile equipment in mining operations"},
		{"Highwall Safety Procedures", "https://www.youtube.com/embed/LD-vNX6_QdE", 420, "Safety", "Critical safety protocols for working near highwalls in surface mining"},
		{"Task Training Overview", "https://www.youtube.com/embed/Bf6pDKcOkmc", 380, "Training", "Comprehensive task-based training for mining operations"},
		{"Blind Spots & Spotter Safety", "https://www.youtube.com/embed/PlDnBQ3Iidw", 340, "Safety", "Understanding blind spots and proper spotter procedures"},
		{"Cranes & Man Lifts Safety", "https://www.youtube.com/embed/zK4BXed9_Hw", 450, "Equipment", "Safe operation of cranes and man lifts in mining environments"},
		{"Dozer Operator Training", "https://www.youtube.com/embed/zlp51JG37lA", 520, "Equipment", "Complete training for dozer operators"},
		{"Excavator Operator Training", "https://www.youtube.com/embed/ZQ2leag_Ucs", 480, "Equipment", "Professional excavator operation training"},
		{"Haul Truck Operator Training", "https://www.youtube.com/embed/p_vsrhxIlR8", 560, "Equipment", "Comprehensive haul truck operation and safety"},
		{"Loader Operator Training", "https://www.youtube.com/embed/-qsG3qDDqMk", 440, "Equipment", "Front-end loader operation and safety procedures"},
		{"Mobile Equipment Inspections", "https://www.youtube.com/embed/ITFCI8A1Afk", 380, "Safety", "Pre-operational inspection procedures for mobile equipment"},
		{"Contractor Responsibilities", "https://www.youtube.com/embed/ITFCI8A1Afk", 360, "Compliance", "Understanding contractor roles and responsibilities"},
		{"Independent Contractors", "https://www.youtube.com/embed/--P3CQY8lYI", 340, "Compliance", "Guidelines for independent contractors in mining"},
		{"Training Requirements", "https://www.youtube.com/embed/UbSm_lAxl5s", 420, "Training", "Mandatory training requirements for mine workers"},
		{"Rules to Live By", "https://www.youtube.com/embed/9I8BzhEydgY", 300, "Safety", "Essential safety rules for all mine workers"},
		{"Mining Environments", "https://www.youtube.com/embed/Xa-SejW-Crc", 400, "Safety", "Understanding different mining environments and hazards"},
		{"Inspections Protocol", "https://www.youtube.com/embed/kTguRi2eoJg", 380, "Safety", "Standard inspection procedures and protocols"},
		{"Water Safety", "https://www.youtube.com/embed/28loa0a22dE", 320, "Safety", "Safety procedures around water in mining operations"},
		{"Statutory Rights", "https://www.youtube.com/embed/SN4Sfuhvs2Y", 360, "Compliance", "Understanding your statutory rights as a mine worker"},
		{"Mine Act Overview", "https://www.youtube.com/embed/-TixTD1jgBY", 420, "Compliance", "Overview of the Mine Safety and Health Act"},
		{"Rules and Procedures", "https://www.youtube.com/embed/T_9-QflQNT4", 380, "Compliance", "Standard operating rules and procedures"},
	}

	log.Println("Seeding default video modules...")
	for _, v := range videos {
		_, err := DB.Exec(
			`INSERT INTO video_modules (title, description, video_url, duration, category, thumbnail, is_active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())`,
			v.title, v.description, v.videoURL, v.duration, v.category, "https://img.youtube.com/vi/"+extractYouTubeID(v.videoURL)+"/maxresdefault.jpg",
		)
		if err != nil {
			return fmt.Errorf("failed to seed video '%s': %w", v.title, err)
		}
	}

	log.Printf("Successfully seeded %d default video modules", len(videos))
	return nil
}

func extractYouTubeID(url string) string {
	// Extract from embed URL: https://www.youtube.com/embed/VIDEO_ID
	if len(url) > 30 && url[:30] == "https://www.youtube.com/embed/" {
		return url[30:]
	}
	return url
}