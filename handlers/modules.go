package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/middleware"
	"MineSafeBackend/models"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func CreateVideoModule(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var moduleData models.VideoModuleCreate
	if err := json.NewDecoder(r.Body).Decode(&moduleData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if moduleData.Title == "" || moduleData.VideoURL == "" {
		respondWithError(w, http.StatusBadRequest, "Title and video URL are required")
		return
	}

	var moduleID int
	err := database.DB.QueryRow(
		`INSERT INTO video_modules (title, description, video_url, duration, category, thumbnail, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id`,
		moduleData.Title, moduleData.Description, moduleData.VideoURL, moduleData.Duration,
		moduleData.Category, moduleData.Thumbnail, supervisorID, time.Now(), time.Now(),
	).Scan(&moduleID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating video module: "+err.Error())
		return
	}

	var module models.VideoModule
	err = database.DB.QueryRow(
		`SELECT id, title, description, video_url, duration, category, thumbnail, is_active, created_by, created_at, updated_at
		 FROM video_modules WHERE id = $1`,
		moduleID,
	).Scan(&module.ID, &module.Title, &module.Description, &module.VideoURL, &module.Duration,
		&module.Category, &module.Thumbnail, &module.IsActive, &module.CreatedBy, &module.CreatedAt, &module.UpdatedAt)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching created module")
		return
	}

	respondWithJSON(w, http.StatusCreated, module)
}

func GetVideoModules(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(
		`SELECT id, title, description, video_url, duration, category, thumbnail, is_active, created_by, created_at, updated_at
		 FROM video_modules WHERE is_active = true ORDER BY created_at DESC`,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	modules := []models.VideoModule{}
	for rows.Next() {
		var module models.VideoModule
		err := rows.Scan(&module.ID, &module.Title, &module.Description, &module.VideoURL, &module.Duration,
			&module.Category, &module.Thumbnail, &module.IsActive, &module.CreatedBy, &module.CreatedAt, &module.UpdatedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning module data")
			return
		}
		modules = append(modules, module)
	}

	respondWithJSON(w, http.StatusOK, modules)
}

func GetVideoModule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	moduleID := vars["id"]

	var module models.VideoModule
	err := database.DB.QueryRow(
		`SELECT id, title, description, video_url, duration, category, thumbnail, is_active, created_by, created_at, updated_at
		 FROM video_modules WHERE id = $1`,
		moduleID,
	).Scan(&module.ID, &module.Title, &module.Description, &module.VideoURL, &module.Duration,
		&module.Category, &module.Thumbnail, &module.IsActive, &module.CreatedBy, &module.CreatedAt, &module.UpdatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Video module not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, module)
}

func SetStarVideo(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	videoID := vars["id"]

	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM video_modules WHERE id = $1)", videoID).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Video module not found")
		return
	}

	today := time.Now().Format("2006-01-02")
	_, err = database.DB.Exec(
		`UPDATE star_videos SET is_active = false 
		 WHERE supervisor_id = $1 AND set_date = $2 AND is_active = true`,
		supervisorID, today,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating existing star video")
		return
	}

	var starID int
	err = database.DB.QueryRow(
		`INSERT INTO star_videos (video_id, supervisor_id, set_date, is_active)
		 VALUES ($1, $2, $3, true)
		 ON CONFLICT (supervisor_id, set_date, is_active) 
		 DO UPDATE SET video_id = $1, is_active = true
		 RETURNING id`,
		videoID, supervisorID, today,
	).Scan(&starID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error setting star video: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Star video set successfully",
		"video_id":   videoID,
		"star_id":    starID,
		"set_date":   today,
		"is_active":  true,
	})
}

func GetStarVideo(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var supervisorID string
	var role string
	err := database.DB.QueryRow(
		"SELECT role, COALESCE(supervisor_id, user_id) FROM users WHERE user_id = $1",
		userID,
	).Scan(&role, &supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching user details")
		return
	}

	if role == "SUPERVISOR" {
		supervisorID = userID
	}

	today := time.Now().Format("2006-01-02")

	var module models.VideoModule
	err = database.DB.QueryRow(
		`SELECT vm.id, vm.title, vm.description, vm.video_url, vm.duration, vm.category, 
		        vm.thumbnail, vm.is_active, vm.created_by, vm.created_at, vm.updated_at
		 FROM video_modules vm
		 JOIN star_videos sv ON vm.id = sv.video_id
		 WHERE sv.supervisor_id = $1 AND sv.set_date = $2 AND sv.is_active = true`,
		supervisorID, today,
	).Scan(&module.ID, &module.Title, &module.Description, &module.VideoURL, &module.Duration,
		&module.Category, &module.Thumbnail, &module.IsActive, &module.CreatedBy, &module.CreatedAt, &module.UpdatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "No star video set for today")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, module)
}

func CreateQuestion(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var questionData models.QuestionCreate
	if err := json.NewDecoder(r.Body).Decode(&questionData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if questionData.Question == "" || len(questionData.Options) == 0 {
		respondWithError(w, http.StatusBadRequest, "Question and options are required")
		return
	}

	var exists bool
	err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM video_modules WHERE id = $1)",
		questionData.VideoID,
	).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Video module not found")
		return
	}

	optionsJSON, err := json.Marshal(questionData.Options)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing options")
		return
	}

	var questionID int
	err = database.DB.QueryRow(
		`INSERT INTO questions (video_id, question, options, answer)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		questionData.VideoID, questionData.Question, optionsJSON, questionData.Answer,
	).Scan(&questionID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating question: "+err.Error())
		return
	}

	var question models.Question
	var optionsStr string
	err = database.DB.QueryRow(
		`SELECT id, video_id, question, options, answer FROM questions WHERE id = $1`,
		questionID,
	).Scan(&question.ID, &question.VideoID, &question.Question, &optionsStr, &question.Answer)

	if err == nil {
		json.Unmarshal([]byte(optionsStr), &question.Options)
	}

	respondWithJSON(w, http.StatusCreated, question)
}

func GetQuestions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID := vars["id"]

	rows, err := database.DB.Query(
		`SELECT id, video_id, question, options, answer FROM questions WHERE video_id = $1`,
		videoID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	questions := []models.Question{}
	for rows.Next() {
		var question models.Question
		var optionsStr string
		err := rows.Scan(&question.ID, &question.VideoID, &question.Question, &optionsStr, &question.Answer)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning question data")
			return
		}
		json.Unmarshal([]byte(optionsStr), &question.Options)
		questions = append(questions, question)
	}

	respondWithJSON(w, http.StatusOK, questions)
}

func SubmitModuleAnswers(w http.ResponseWriter, r *http.Request) {
	minerID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var submission models.ModuleAnswer
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	rows, err := database.DB.Query(
		`SELECT id, answer FROM questions WHERE video_id = $1 ORDER BY id`,
		submission.VideoID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	correctAnswers := make(map[int]int)
	questionIDs := []int{}
	for rows.Next() {
		var qID, answer int
		rows.Scan(&qID, &answer)
		correctAnswers[qID] = answer
		questionIDs = append(questionIDs, qID)
	}

	totalQuestions := len(correctAnswers)
	if totalQuestions == 0 {
		respondWithError(w, http.StatusBadRequest, "No questions found for this video")
		return
	}

	if len(submission.Answers) != totalQuestions {
		respondWithError(w, http.StatusBadRequest, "Answer count doesn't match question count")
		return
	}

	// Calculate score
	score := 0
	for i, qID := range questionIDs {
		if i < len(submission.Answers) && submission.Answers[i] == correctAnswers[qID] {
			score++
		}
	}

	var completionID int
	err = database.DB.QueryRow(
		`SELECT id FROM module_completions 
		 WHERE miner_id = $1 AND video_id = $2 AND completed_at::date = CURRENT_DATE`,
		minerID, submission.VideoID,
	).Scan(&completionID)

	if err == nil {
		_, err = database.DB.Exec(
			`UPDATE module_completions SET score = $1, total_questions = $2, completed_at = NOW()
			 WHERE id = $3`,
			score, totalQuestions, completionID,
		)
	} else {
		err = database.DB.QueryRow(
			`INSERT INTO module_completions (miner_id, video_id, score, total_questions, completed_at)
			 VALUES ($1, $2, $3, $4, NOW())
			 RETURNING id`,
			minerID, submission.VideoID, score, totalQuestions,
		).Scan(&completionID)
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error recording completion: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"completion_id":    completionID,
		"score":            score,
		"total_questions":  totalQuestions,
		"percentage":       float64(score) / float64(totalQuestions) * 100,
		"message":          "Module completed successfully",
	})
}

// Helper function to extract YouTube video ID from various YouTube URL formats
func extractYouTubeID(url string) string {
	// Handle youtube.com/watch?v=VIDEO_ID
	if len(url) > 32 && url[0:32] == "https://www.youtube.com/watch?v=" {
		return url[32:]
	}
	if len(url) > 31 && url[0:31] == "https://youtube.com/watch?v=" {
		return url[31:]
	}
	// Handle youtu.be/VIDEO_ID
	if len(url) > 17 && url[0:17] == "https://youtu.be/" {
		return url[17:]
	}
	// Handle youtube.com/embed/VIDEO_ID
	if len(url) > 30 && url[0:30] == "https://www.youtube.com/embed/" {
		return url[30:]
	}
	// If it's already just an ID, return it
	return url
}

// Convert YouTube URL to embed URL
func convertToYouTubeEmbed(url string) string {
	videoID := extractYouTubeID(url)
	return "https://www.youtube.com/embed/" + videoID
}
