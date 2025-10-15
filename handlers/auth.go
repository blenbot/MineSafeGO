package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/middleware"
	"MineSafeBackend/models"
	"database/sql"
	"encoding/json"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type AuthResponse struct {
	Token          string       `json:"token"`
	UserID         string       `json:"user_id"`
	Role           string       `json:"role"`
	User           *models.User `json:"user"`
	SupervisorName string       `json:"supervisor_name,omitempty"`
	OrganizationID string       `json:"organization_id,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func SupervisorSignup(w http.ResponseWriter, r *http.Request) {
	var signup models.UserSignup
	if err := json.NewDecoder(r.Body).Decode(&signup); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate required fields
	if signup.Name == "" || signup.Email == "" || signup.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Name, email, and password are required")
		return
	}

	// Check if email already exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", signup.Email).Scan(&exists)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if exists {
		respondWithError(w, http.StatusConflict, "Email already registered")
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing password")
		return
	}

	// Create user
	user, err := models.NewUser(signup.Name, signup.Email, signup.Phone, string(hashedPassword), 
		signup.MiningSite, signup.Location, models.RoleSupervisor, nil)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Insert into database
	err = database.DB.QueryRow(
		`INSERT INTO users (user_id, name, email, phone, password, role, mining_site, location, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		user.UserID, user.Name, user.Email, user.Phone, user.Password, user.Role,
		user.MiningSite, user.Location, user.CreatedAt, user.UpdatedAt,
	).Scan(&user.ID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating user: "+err.Error())
		return
	}

	// Generate token
	token, err := middleware.GenerateToken(user.UserID, string(user.Role))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating token")
		return
	}

	user.Password = "" // Don't send password back
	respondWithJSON(w, http.StatusCreated, AuthResponse{
		Token:  token,
		UserID: user.UserID,
		Role:   string(user.Role),
		User:   user,
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	var login models.UserLogin
	if err := json.NewDecoder(r.Body).Decode(&login); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if login.Email == "" || login.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	// Find user by email
	var user models.User
	err := database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, password, role, mining_site, location, supervisor_id, created_at, updated_at
		 FROM users WHERE email = $1`,
		login.Email,
	).Scan(&user.ID, &user.UserID, &user.Name, &user.Email, &user.Phone, &user.Password,
		&user.Role, &user.MiningSite, &user.Location, &user.SupervisorID, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(login.Password)); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Generate token
	token, err := middleware.GenerateToken(user.UserID, string(user.Role))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating token")
		return
	}

	// If user is a miner, fetch supervisor name
	supervisorName := ""
	if user.Role == models.RoleMiner && user.SupervisorID != nil {
		database.DB.QueryRow(
			"SELECT name FROM users WHERE user_id = $1",
			*user.SupervisorID,
		).Scan(&supervisorName)
	}

	user.Password = "" // Don't send password back
	respondWithJSON(w, http.StatusOK, AuthResponse{
		Token:          token,
		UserID:         user.UserID,
		Role:           string(user.Role),
		User:           &user,
		SupervisorName: supervisorName,
		OrganizationID: user.MiningSite, // mining_site is the organization identifier
	})
}

func GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	err := database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, supervisor_id, created_at, updated_at
		 FROM users WHERE user_id = $1`,
		userID,
	).Scan(&user.ID, &user.UserID, &user.Name, &user.Email, &user.Phone,
		&user.Role, &user.MiningSite, &user.Location, &user.SupervisorID, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{Error: message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}
