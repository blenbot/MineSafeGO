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

	// 3. Fetch miner by email
	usr, err := database.GetUserByEmail(ctx, req.Email, req.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 4. Compare password provided vs stored hash
	if err := bcrypt.CompareHashAndPassword([]byte(usr.Password), []byte(req.Password)); err != nil {
		// Wrong password
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// 5. Ensure miner is assigned to a supervisor
	if usr.SupervisorID == nil || *usr.SupervisorID == "" {
		http.Error(w, "Miner is not assigned to a supervisor", http.StatusConflict)
		return
	}

	// 6. Check supervisor exists + get name
	supervisorName, err := database.GetSupervisorNameByUserID(ctx, *usr.SupervisorID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Supervisor not found", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// 7. Generate JWT using your existing auth.go
	token, err := middleware.GenerateToken(usr.UserID, "MINER")
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// 8. Build response
	resp := MinerLoginResponse{
		Token:          token,
		MinerID:        usr.UserID,
		MinerName:      usr.Name,
		SupervisorName: supervisorName,
		MiningSite:     *usr.MiningSite,
	}

	// 9. Send JSON response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
		return
	}
}
