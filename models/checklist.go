package models

import (
	"time"
)

// PreStartChecklistItem represents a pre-start checklist item created by supervisor
type PreStartChecklistItem struct {
	ID           int       `json:"id" db:"id"`
	SupervisorID string    `json:"supervisor_id" db:"supervisor_id"`
	Title        string    `json:"title" db:"title"`
	Description  string    `json:"description" db:"description"`
	IsDefault    bool      `json:"is_default" db:"is_default"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// PPEChecklistItem represents a PPE checklist item created by supervisor
type PPEChecklistItem struct {
	ID           int       `json:"id" db:"id"`
	SupervisorID string    `json:"supervisor_id" db:"supervisor_id"`
	Title        string    `json:"title" db:"title"`
	Description  string    `json:"description" db:"description"`
	IsDefault    bool      `json:"is_default" db:"is_default"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// PreStartChecklistCompletion tracks user completion of pre-start checklist items
type PreStartChecklistCompletion struct {
	ID          int       `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	ItemID      int       `json:"item_id" db:"item_id"`
	IsCompleted bool      `json:"is_completed" db:"is_completed"`
	CompletedAt time.Time `json:"completed_at" db:"completed_at"`
	Date        string    `json:"date" db:"date"` // YYYY-MM-DD format for daily tracking
}

// PPEChecklistCompletion tracks user completion of PPE checklist items
type PPEChecklistCompletion struct {
	ID          int       `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	ItemID      int       `json:"item_id" db:"item_id"`
	IsCompleted bool      `json:"is_completed" db:"is_completed"`
	CompletedAt time.Time `json:"completed_at" db:"completed_at"`
	Date        string    `json:"date" db:"date"` // YYYY-MM-DD format for daily tracking
}

// ChecklistItemCreate is used for creating new checklist items
type ChecklistItemCreate struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// ChecklistCompletionUpdate is used for updating checklist completion status
type ChecklistCompletionUpdate struct {
	ItemID      int  `json:"item_id"`
	IsCompleted bool `json:"is_completed"`
}

// ChecklistItemWithStatus combines item info with completion status for response
type ChecklistItemWithStatus struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	IsCompleted bool       `json:"is_completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// CalendarStreakResponse is the response for quiz attempt dates and streak
type CalendarStreakResponse struct {
	UserID        string   `json:"user_id"`
	UserName      string   `json:"user_name"`
	CurrentStreak int      `json:"current_streak"`
	LongestStreak int      `json:"longest_streak"`
	TotalDays     int      `json:"total_days"`
	AttemptDates  []string `json:"attempt_dates"` // Array of dates in YYYY-MM-DD format
}
