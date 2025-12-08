package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"MineSafeBackend/database"
	"MineSafeBackend/middleware"

	"golang.org/x/crypto/bcrypt"
)

// Request shape coming from the app
type MinerLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Response we send back to the app
type MinerLoginResponse struct {
	Token          string `json:"token"`
	MinerID        string `json:"miner_id"`
	MinerName      string `json:"miner_name"`
	SupervisorName string `json:"supervisor_name"`
	MiningSite     string `json:"location"`
}

func MinerAppLogin(w http.ResponseWriter, r *http.Request) {
	// 1. Decode JSON body into request struct
	var req MinerLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	// 2. Basic input validation
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 3. Fetch user by email
	result, err := database.GetUserByEmail(ctx, req.Email, req.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 4. Handle different user types based on role
	var userID, userName, role, miningSite string
	var supervisorName string

	// Type assertion based on role
	switch req.Role {
	case "MINER", "OPERATOR":
		usr, ok := result.(*database.User)
		if !ok {
			http.Error(w, "Invalid user type for role", http.StatusInternalServerError)
			return
		}

		// 5. Compare password provided vs stored hash
		if err := bcrypt.CompareHashAndPassword([]byte(usr.Password), []byte(req.Password)); err != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		// 6. Ensure miner/operator is assigned to a supervisor
		if usr.SupervisorID == nil || *usr.SupervisorID == "" {
			http.Error(w, "User is not assigned to a supervisor", http.StatusConflict)
			return
		}

		// 7. Check supervisor exists + get name
		supervisorName, err = database.GetSupervisorNameByUserID(ctx, *usr.SupervisorID)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Supervisor not found", http.StatusInternalServerError)
				return
			}
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		userID = usr.UserID
		userName = usr.Name
		role = usr.Role
		if usr.MiningSite != nil {
			miningSite = *usr.MiningSite
		}

	case "SUPERVISOR":
		sup, ok := result.(*database.Supervisor)
		if !ok {
			http.Error(w, "Invalid user type for role", http.StatusInternalServerError)
			return
		}

		// 5. Compare password provided vs stored hash
		if err := bcrypt.CompareHashAndPassword([]byte(sup.Password), []byte(req.Password)); err != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		userID = sup.UserID
		userName = sup.Name
		role = sup.Role
		supervisorName = sup.Name // Supervisor's own name
		if sup.MiningSite != nil {
			miningSite = *sup.MiningSite
		}

	default:
		http.Error(w, "Invalid role specified", http.StatusBadRequest)
		return
	}

	// 8. Generate JWT
	token, err := middleware.GenerateToken(userID, role)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// 9. Build response
	resp := MinerLoginResponse{
		Token:          token,
		MinerID:        userID,
		MinerName:      userName,
		SupervisorName: supervisorName,
		MiningSite:     miningSite,
	}

	// 9. Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}
