package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSupervisor Role = "SUPERVISOR"
	RoleMiner      Role = "MINER"
	RoleAdmin      Role = "ADMIN"
)

type User struct {
	ID           int       `json:"id" db:"id"`
	UserID       string    `json:"user_id" db:"user_id"`
	Name         string    `json:"name" db:"name"`
	Email        string    `json:"email" db:"email"`
	Phone        string    `json:"phone" db:"phone"`
	Password     string    `json:"-" db:"password"`
	Role         Role      `json:"role" db:"role"`
	MiningSite   string    `json:"mining_site" db:"mining_site"`
	Location     string    `json:"location" db:"location"`
	SupervisorID *string   `json:"supervisor_id,omitempty" db:"supervisor_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type UserLogin struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserSignup struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Password   string `json:"password"`
	MiningSite string `json:"mining_site"`
	Location   string `json:"location"`
}

type MinerCreate struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

func NewUser(name, email, phone, password, miningSite, location string, role Role, supervisorID *string) (*User, error) {
	if name == "" || email == "" || password == "" {
		return nil, errors.New("invalid user details: name, email, and password are required")
	}

	var userID string
	switch role {
	case RoleSupervisor:
		userID = "SUP-" + uuid.New().String()
	case RoleMiner:
		userID = "MIN-" + uuid.New().String()
	case RoleAdmin:
		userID = "ADM-" + uuid.New().String()
	default:
		return nil, errors.New("invalid role")
	}

	user := &User{
		UserID:       userID,
		Name:         name,
		Email:        email,
		Phone:        phone,
		Password:     password,
		Role:         role,
		MiningSite:   miningSite,
		Location:     location,
		SupervisorID: supervisorID,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	return user, nil
}
