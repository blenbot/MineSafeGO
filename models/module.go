package models

import (
	"time"
)

type VideoModule struct {
	ID          int       `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	VideoURL    string    `json:"video_url" db:"video_url"`
	Duration    int       `json:"duration" db:"duration"` // Duration in seconds
	Category    string    `json:"category" db:"category"`
	Thumbnail   string    `json:"thumbnail" db:"thumbnail"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedBy   *string   `json:"created_by,omitempty" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type StarVideo struct {
	ID           int       `json:"id" db:"id"`
	VideoID      int       `json:"video_id" db:"video_id"`
	SupervisorID string    `json:"supervisor_id" db:"supervisor_id"`
	SetDate      time.Time `json:"set_date" db:"set_date"`
	IsActive     bool      `json:"is_active" db:"is_active"`
}

type Question struct {
	ID       int      `json:"id" db:"id"`
	VideoID  int      `json:"video_id" db:"video_id"`
	Question string   `json:"question" db:"question"`
	Options  []string `json:"options" db:"options"` // Stored as JSON array
	Answer   int      `json:"answer" db:"answer"`   // Index of correct answer
}

type ModuleCompletion struct {
	ID           int       `json:"id" db:"id"`
	MinerID      string    `json:"miner_id" db:"miner_id"`
	VideoID      int       `json:"video_id" db:"video_id"`
	CompletedAt  time.Time `json:"completed_at" db:"completed_at"`
	Score        int       `json:"score" db:"score"`
	TotalQuestions int     `json:"total_questions" db:"total_questions"`
}

type LearningStreak struct {
	MinerID       string    `json:"miner_id"`
	MinerName     string    `json:"miner_name"`
	CurrentStreak int       `json:"current_streak"`
	LastCompleted time.Time `json:"last_completed"`
	TotalModules  int       `json:"total_modules"`
}

type VideoModuleCreate struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	VideoURL    string `json:"video_url"`
	Duration    int    `json:"duration"`
	Category    string `json:"category"`
	Thumbnail   string `json:"thumbnail"`
	VideoType   string `json:"video_type"` // "youtube", "upload", or "url"
}

type QuestionCreate struct {
	VideoID  int      `json:"video_id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Answer   int      `json:"answer"`
}

type ModuleAnswer struct {
	VideoID int   `json:"video_id"`
	Answers []int `json:"answers"` // Array of selected answer indices
}
