package database

import "time"

type User struct {
	ID           int64
	UserID       string
	Name         string
	Email        string
	Phone        *string
	Password     string
	Role         string
	MiningSite   *string
	Location     *string
	SupervisorID *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Supervisor struct {
	ID         int64
	UserID     string
	Name       string
	Email      string
	Phone      *string
	Password   string
	Role       string
	MiningSite *string
	Location   *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type MinerMetrics struct {
	TotalCompletions int
	AverageScore     *float64
	LastCompletedAt  *time.Time
}

type OperatorMetrics struct {
	TotalCompletions int
	AverageScore     *float64
	LastCompletedAt  *time.Time
}
