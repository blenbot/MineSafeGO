package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/middleware"
	"MineSafeBackend/models"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

func CreateMiner(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var minerData models.MinerCreate
	if err := json.NewDecoder(r.Body).Decode(&minerData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if minerData.Name == "" || minerData.Email == "" || minerData.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Name, email, and password are required")
		return
	}

	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", minerData.Email).Scan(&exists)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if exists {
		respondWithError(w, http.StatusConflict, "Email already registered")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(minerData.Password), bcrypt.DefaultCost)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing password")
		return
	}

	// Handle both phone and phone_number fields
	phone := minerData.Phone
	if phone == "" && minerData.PhoneNumber != "" {
		phone = minerData.PhoneNumber
	}

	var miningSite, location string
	err = database.DB.QueryRow(
		"SELECT mining_site, location FROM users WHERE user_id = $1",
		supervisorID,
	).Scan(&miningSite, &location)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching supervisor details")
		return
	}

	miner, err := models.NewUser(minerData.Name, minerData.Email, phone, 
		string(hashedPassword), miningSite, location, models.RoleMiner, &supervisorID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	err = database.DB.QueryRow(
		`INSERT INTO users (user_id, name, email, phone, password, role, mining_site, location, supervisor_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		miner.UserID, miner.Name, miner.Email, miner.Phone, miner.Password, miner.Role,
		miner.MiningSite, miner.Location, miner.SupervisorID, miner.CreatedAt, miner.UpdatedAt,
	).Scan(&miner.ID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating miner: "+err.Error())
		return
	}

	miner.Password = ""
	respondWithJSON(w, http.StatusCreated, miner)
}

func GetMiners(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	rows, err := database.DB.Query(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, supervisor_id, created_at, updated_at
		 FROM users WHERE supervisor_id = $1 ORDER BY created_at DESC`,
		supervisorID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	miners := []models.User{}
	for rows.Next() {
		var miner models.User
		err := rows.Scan(&miner.ID, &miner.UserID, &miner.Name, &miner.Email, &miner.Phone,
			&miner.Role, &miner.MiningSite, &miner.Location, &miner.SupervisorID, &miner.CreatedAt, &miner.UpdatedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning miner data")
			return
		}
		miners = append(miners, miner)
	}

	respondWithJSON(w, http.StatusOK, miners)
}

func GetMiner(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	minerID := vars["id"]

	var miner models.User
	err := database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, supervisor_id, created_at, updated_at
		 FROM users WHERE user_id = $1 AND supervisor_id = $2`,
		minerID, supervisorID,
	).Scan(&miner.ID, &miner.UserID, &miner.Name, &miner.Email, &miner.Phone,
		&miner.Role, &miner.MiningSite, &miner.Location, &miner.SupervisorID, &miner.CreatedAt, &miner.UpdatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Miner not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, miner)
}

func UpdateMiner(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	minerID := vars["id"]

	var updateData struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		Phone       string `json:"phone"`
		PhoneNumber string `json:"phone_number"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var exists bool
	err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND supervisor_id = $2)",
		minerID, supervisorID,
	).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Miner not found")
		return
	}

	phone := updateData.Phone
	if phone == "" && updateData.PhoneNumber != "" {
		phone = updateData.PhoneNumber
	}

	result, err := database.DB.Exec(
		`UPDATE users SET name = $1, email = $2, phone = $3, updated_at = NOW()
		 WHERE user_id = $4 AND supervisor_id = $5`,
		updateData.Name, updateData.Email, phone, minerID, supervisorID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating miner")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Miner not found")
		return
	}

	var miner models.User
	err = database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, supervisor_id, created_at, updated_at
		 FROM users WHERE user_id = $1`,
		minerID,
	).Scan(&miner.ID, &miner.UserID, &miner.Name, &miner.Email, &miner.Phone,
		&miner.Role, &miner.MiningSite, &miner.Location, &miner.SupervisorID, &miner.CreatedAt, &miner.UpdatedAt)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching updated miner")
		return
	}

	respondWithJSON(w, http.StatusOK, miner)
}

func DeleteMiner(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	minerID := vars["id"]

	result, err := database.DB.Exec(
		"DELETE FROM users WHERE user_id = $1 AND supervisor_id = $2",
		minerID, supervisorID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error deleting miner")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Miner not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Miner deleted successfully"})
}
