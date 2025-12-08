package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/middleware"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type UserTagsResponse struct {
	Success bool     `json:"success"`
	Tags    []string `json:"tags"`
}

type UpdateTagsRequest struct {
	Tags []string `json:"tags"`
}

// GetUserTags - GET /api/user/tags
func GetUserTags(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var tagsJSON []byte
	err := database.DB.QueryRow("SELECT COALESCE(tags, '[]'::jsonb) FROM users WHERE user_id = $1", userID).Scan(&tagsJSON)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	var tags []string
	json.Unmarshal(tagsJSON, &tags)
	if tags == nil {
		tags = []string{}
	}

	respondWithJSON(w, http.StatusOK, UserTagsResponse{
		Success: true,
		Tags:    tags,
	})
}

// UpdateUserTags - PUT /api/user/tags
func UpdateUserTags(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Tags == nil {
		req.Tags = []string{}
	}

	tagsJSON, _ := json.Marshal(req.Tags)

	_, err := database.DB.Exec("UPDATE users SET tags = $1, updated_at = NOW() WHERE user_id = $2", tagsJSON, userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update tags: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, UserTagsResponse{
		Success: true,
		Tags:    req.Tags,
	})
}

// ==================== USER PROFILE ====================

type UserProfileResponse struct {
	UserID            string    `json:"user_id"`
	Name              string    `json:"name"`
	Email             string    `json:"email"`
	Phone             string    `json:"phone"`
	SupervisorName    string    `json:"supervisor_name"`
	MiningSite        string    `json:"mining_site"`
	ProfilePictureURL string    `json:"profile_picture_url,omitempty"`
	Tags              []string  `json:"tags"`
	CreatedAt         time.Time `json:"created_at"`
}

type UpdateProfileRequest struct {
	Name  string `json:"name,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// GetUserProfile - GET /api/app/profile
func GetUserProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var profile UserProfileResponse
	var supervisorID sql.NullString
	var phone, miningSite, profilePic sql.NullString
	var tagsJSON []byte

	err := database.DB.QueryRow(`
		SELECT user_id, name, email, phone, mining_site, supervisor_id, 
			   profile_picture_url, COALESCE(tags, '[]'::jsonb), created_at
		FROM users WHERE user_id = $1
	`, userID).Scan(&profile.UserID, &profile.Name, &profile.Email, &phone,
		&miningSite, &supervisorID, &profilePic, &tagsJSON, &profile.CreatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	if phone.Valid {
		profile.Phone = phone.String
	}
	if miningSite.Valid {
		profile.MiningSite = miningSite.String
	}
	if profilePic.Valid {
		profile.ProfilePictureURL = profilePic.String
	}

	json.Unmarshal(tagsJSON, &profile.Tags)
	if profile.Tags == nil {
		profile.Tags = []string{}
	}

	// Get supervisor name if assigned
	if supervisorID.Valid && supervisorID.String != "" {
		database.DB.QueryRow("SELECT name FROM users WHERE user_id = $1", supervisorID.String).Scan(&profile.SupervisorName)
	}

	respondWithJSON(w, http.StatusOK, profile)
}

// UpdateUserProfile - PUT /api/app/profile
func UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.Name != "" {
		updates = append(updates, "name = $"+string(rune('0'+argCount)))
		args = append(args, req.Name)
		argCount++
	}
	if req.Phone != "" {
		updates = append(updates, "phone = $"+string(rune('0'+argCount)))
		args = append(args, req.Phone)
		argCount++
	}

	if len(updates) == 0 {
		respondWithError(w, http.StatusBadRequest, "No fields to update")
		return
	}

	query := "UPDATE users SET "
	for i, u := range updates {
		if i > 0 {
			query += ", "
		}
		query += u
	}
	query += ", updated_at = NOW() WHERE user_id = $" + string(rune('0'+argCount))
	args = append(args, userID)

	_, err := database.DB.Exec(query, args...)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update profile: "+err.Error())
		return
	}

	// Return updated profile
	GetUserProfile(w, r)
}

// UploadProfilePicture - POST /api/app/profile/picture (multipart/form-data)
func UploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, handler, err := r.FormFile("picture")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Picture file is required")
		return
	}
	defer file.Close()

	// Validate file type
	ext := filepath.Ext(handler.Filename)
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}
	if !allowedExts[ext] {
		respondWithError(w, http.StatusBadRequest, "Only JPG, PNG, and GIF files are allowed")
		return
	}

	// Create uploads directory
	uploadsDir := "uploads/profile_pictures"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Generate unique filename
	fileName := uuid.New().String() + ext
	filePath := filepath.Join(uploadsDir, fileName)

	// Save file
	destFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save picture")
		return
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save picture")
		return
	}

	// Generate URL
	pictureURL := "/uploads/profile_pictures/" + fileName

	// Update user profile
	_, err = database.DB.Exec("UPDATE users SET profile_picture_url = $1, updated_at = NOW() WHERE user_id = $2",
		pictureURL, userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success":             true,
		"profile_picture_url": pictureURL,
		"message":             "Profile picture uploaded successfully",
	})
}
