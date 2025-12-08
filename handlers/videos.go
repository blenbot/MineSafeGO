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
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// ==================== VIDEO FEED RESPONSES ====================

type VideoFeedItem struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	VideoURL     string   `json:"video_url"`
	ThumbnailURL string   `json:"thumbnail_url"`
	Tags         []string `json:"tags"`
	Likes        int      `json:"likes"`
	Dislikes     int      `json:"dislikes"`
	UserLiked    bool     `json:"user_liked"`
	UserDisliked bool     `json:"user_disliked"`
	HasQuiz      bool     `json:"has_quiz"`
}

type VideoFeedResponse struct {
	Videos  []VideoFeedItem `json:"videos"`
	HasMore bool            `json:"has_more"`
	Total   int             `json:"total"`
}

// ==================== VIDEO FEED ENDPOINTS ====================

// GetVideoFeed - GET /api/videos/feed?page=1&limit=10
func GetVideoFeed(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse pagination params
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Get total count
	var total int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM video_modules WHERE is_active = true").Scan(&total)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Get videos with user reaction status
	rows, err := database.DB.Query(`
		SELECT 
			vm.id, vm.title, vm.video_url, vm.thumbnail, 
			COALESCE(vm.tags, '[]'::jsonb) as tags,
			COALESCE(vm.likes_count, 0) as likes,
			COALESCE(vm.dislikes_count, 0) as dislikes,
			COALESCE(vr.reaction_type, '') as user_reaction,
			EXISTS(SELECT 1 FROM questions q WHERE q.video_id = vm.id) OR 
			EXISTS(SELECT 1 FROM quizzes qz WHERE qz.video_id = vm.id) as has_quiz
		FROM video_modules vm
		LEFT JOIN video_reactions vr ON vm.id = vr.video_id AND vr.user_id = $1
		WHERE vm.is_active = true
		ORDER BY vm.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	videos := []VideoFeedItem{}
	for rows.Next() {
		var video VideoFeedItem
		var tagsJSON []byte
		var userReaction string
		var thumbnail sql.NullString
		var idInt int

		err := rows.Scan(&idInt, &video.Title, &video.VideoURL, &thumbnail, &tagsJSON,
			&video.Likes, &video.Dislikes, &userReaction, &video.HasQuiz)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning video: "+err.Error())
			return
		}

		video.ID = strconv.Itoa(idInt)
		if thumbnail.Valid {
			video.ThumbnailURL = thumbnail.String
		}

		// Parse tags
		json.Unmarshal(tagsJSON, &video.Tags)
		if video.Tags == nil {
			video.Tags = []string{}
		}

		video.UserLiked = userReaction == "like"
		video.UserDisliked = userReaction == "dislike"

		videos = append(videos, video)
	}

	hasMore := offset+limit < total

	respondWithJSON(w, http.StatusOK, VideoFeedResponse{
		Videos:  videos,
		HasMore: hasMore,
		Total:   total,
	})
}

// GetRecommendedVideos - GET /api/videos/recommended?tags=PPE,safety,HEMI
func GetRecommendedVideos(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tagsParam := r.URL.Query().Get("tags")
	var tags []string

	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	} else {
		// If no tags provided, get user's tags
		var userTagsJSON []byte
		err := database.DB.QueryRow("SELECT COALESCE(tags, '[]'::jsonb) FROM users WHERE user_id = $1", userID).Scan(&userTagsJSON)
		if err == nil {
			json.Unmarshal(userTagsJSON, &tags)
		}
	}

	if len(tags) == 0 {
		tags = []string{} // Empty array for query
	}

	// Build tags JSON array for query
	tagsJSON, _ := json.Marshal(tags)

	// Get videos matching any of the tags
	rows, err := database.DB.Query(`
		SELECT 
			vm.id, vm.title, vm.video_url, vm.thumbnail, 
			COALESCE(vm.tags, '[]'::jsonb) as tags,
			COALESCE(vm.likes_count, 0) as likes,
			COALESCE(vm.dislikes_count, 0) as dislikes,
			COALESCE(vr.reaction_type, '') as user_reaction,
			EXISTS(SELECT 1 FROM questions q WHERE q.video_id = vm.id) OR 
			EXISTS(SELECT 1 FROM quizzes qz WHERE qz.video_id = vm.id) as has_quiz
		FROM video_modules vm
		LEFT JOIN video_reactions vr ON vm.id = vr.video_id AND vr.user_id = $1
		WHERE vm.is_active = true
		AND (
			$2::jsonb = '[]'::jsonb OR
			vm.tags ?| ARRAY(SELECT jsonb_array_elements_text($2::jsonb))
		)
		ORDER BY vm.likes_count DESC, vm.created_at DESC
		LIMIT 20
	`, userID, tagsJSON)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	videos := []VideoFeedItem{}
	for rows.Next() {
		var video VideoFeedItem
		var tagsJSONResult []byte
		var userReaction string
		var thumbnail sql.NullString
		var idInt int

		err := rows.Scan(&idInt, &video.Title, &video.VideoURL, &thumbnail, &tagsJSONResult,
			&video.Likes, &video.Dislikes, &userReaction, &video.HasQuiz)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning video: "+err.Error())
			return
		}

		video.ID = strconv.Itoa(idInt)
		if thumbnail.Valid {
			video.ThumbnailURL = thumbnail.String
		}

		json.Unmarshal(tagsJSONResult, &video.Tags)
		if video.Tags == nil {
			video.Tags = []string{}
		}

		video.UserLiked = userReaction == "like"
		video.UserDisliked = userReaction == "dislike"

		videos = append(videos, video)
	}

	respondWithJSON(w, http.StatusOK, VideoFeedResponse{
		Videos:  videos,
		HasMore: false,
		Total:   len(videos),
	})
}

// LikeVideo - POST /api/videos/{id}/like
func LikeVideo(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	videoID := vars["id"]

	// Check if video exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM video_modules WHERE id = $1)", videoID).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Video not found")
		return
	}

	// Check current reaction
	var currentReaction sql.NullString
	database.DB.QueryRow("SELECT reaction_type FROM video_reactions WHERE user_id = $1 AND video_id = $2",
		userID, videoID).Scan(&currentReaction)

	tx, err := database.DB.Begin()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback()

	if currentReaction.Valid && currentReaction.String == "like" {
		// Remove like (toggle off)
		_, err = tx.Exec("DELETE FROM video_reactions WHERE user_id = $1 AND video_id = $2", userID, videoID)
		if err == nil {
			_, err = tx.Exec("UPDATE video_modules SET likes_count = GREATEST(likes_count - 1, 0) WHERE id = $1", videoID)
		}
	} else {
		// If had dislike, remove it first
		if currentReaction.Valid && currentReaction.String == "dislike" {
			_, err = tx.Exec("UPDATE video_modules SET dislikes_count = GREATEST(dislikes_count - 1, 0) WHERE id = $1", videoID)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Database error")
				return
			}
		}

		// Upsert like
		_, err = tx.Exec(`
			INSERT INTO video_reactions (user_id, video_id, reaction_type, created_at)
			VALUES ($1, $2, 'like', NOW())
			ON CONFLICT (user_id, video_id) DO UPDATE SET reaction_type = 'like', created_at = NOW()
		`, userID, videoID)

		if err == nil {
			if !currentReaction.Valid || currentReaction.String != "like" {
				_, err = tx.Exec("UPDATE video_modules SET likes_count = likes_count + 1 WHERE id = $1", videoID)
			}
		}
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	if err = tx.Commit(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Reaction updated",
	})
}

// DislikeVideo - POST /api/videos/{id}/dislike
func DislikeVideo(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	videoID := vars["id"]

	// Check if video exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM video_modules WHERE id = $1)", videoID).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Video not found")
		return
	}

	// Check current reaction
	var currentReaction sql.NullString
	database.DB.QueryRow("SELECT reaction_type FROM video_reactions WHERE user_id = $1 AND video_id = $2",
		userID, videoID).Scan(&currentReaction)

	tx, err := database.DB.Begin()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback()

	if currentReaction.Valid && currentReaction.String == "dislike" {
		// Remove dislike (toggle off)
		_, err = tx.Exec("DELETE FROM video_reactions WHERE user_id = $1 AND video_id = $2", userID, videoID)
		if err == nil {
			_, err = tx.Exec("UPDATE video_modules SET dislikes_count = GREATEST(dislikes_count - 1, 0) WHERE id = $1", videoID)
		}
	} else {
		// If had like, remove it first
		if currentReaction.Valid && currentReaction.String == "like" {
			_, err = tx.Exec("UPDATE video_modules SET likes_count = GREATEST(likes_count - 1, 0) WHERE id = $1", videoID)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, "Database error")
				return
			}
		}

		// Upsert dislike
		_, err = tx.Exec(`
			INSERT INTO video_reactions (user_id, video_id, reaction_type, created_at)
			VALUES ($1, $2, 'dislike', NOW())
			ON CONFLICT (user_id, video_id) DO UPDATE SET reaction_type = 'dislike', created_at = NOW()
		`, userID, videoID)

		if err == nil {
			if !currentReaction.Valid || currentReaction.String != "dislike" {
				_, err = tx.Exec("UPDATE video_modules SET dislikes_count = dislikes_count + 1 WHERE id = $1", videoID)
			}
		}
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	if err = tx.Commit(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Reaction updated",
	})
}

// ==================== VIDEO UPLOAD ====================

// UploadVideo - POST /api/videos/upload (multipart/form-data)
func UploadVideo(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse multipart form (max 100MB)
	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	// Get form fields
	title := r.FormValue("title")
	if title == "" {
		respondWithError(w, http.StatusBadRequest, "Title is required")
		return
	}

	tagsStr := r.FormValue("tags")
	var tags []string
	if tagsStr != "" {
		// Try parsing as JSON array first
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			// Fall back to comma-separated
			tags = strings.Split(tagsStr, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
		}
	}

	quizStr := r.FormValue("quiz")

	// Get video file
	file, handler, err := r.FormFile("mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video file is required")
		return
	}
	defer file.Close()

	// Validate file type
	if !strings.HasSuffix(strings.ToLower(handler.Filename), ".mp4") {
		respondWithError(w, http.StatusBadRequest, "Only MP4 files are allowed")
		return
	}

	// Create uploads directory if not exists
	uploadsDir := "uploads/videos"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Generate unique filename
	videoFileName := uuid.New().String() + ".mp4"
	videoFilePath := filepath.Join(uploadsDir, videoFileName)

	// Save file
	destFile, err := os.Create(videoFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save video")
		return
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save video")
		return
	}

	// Generate video URL (relative path or full URL based on your setup)
	videoURL := "/uploads/videos/" + videoFileName

	// Convert tags to JSON
	tagsJSON, _ := json.Marshal(tags)

	// Insert video module
	var videoID int
	err = database.DB.QueryRow(`
		INSERT INTO video_modules (title, video_url, tags, created_by, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW(), NOW())
		RETURNING id
	`, title, videoURL, tagsJSON, userID).Scan(&videoID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save video to database: "+err.Error())
		return
	}

	// If quiz provided, create quiz and questions
	if quizStr != "" {
		var quizData struct {
			Questions []struct {
				Question string   `json:"question"`
				Options  []string `json:"options"`
				Correct  int      `json:"correct"`
			} `json:"questions"`
		}

		if err := json.Unmarshal([]byte(quizStr), &quizData); err == nil && len(quizData.Questions) > 0 {
			// Create quiz
			var quizID int
			quizTitle := "Quiz: " + title
			err = database.DB.QueryRow(`
				INSERT INTO quizzes (video_id, title, tags, created_by, created_at, updated_at)
				VALUES ($1, $2, $3, $4, NOW(), NOW())
				RETURNING id
			`, videoID, quizTitle, tagsJSON, userID).Scan(&quizID)

			if err == nil {
				// Insert questions
				for _, q := range quizData.Questions {
					optionsJSON, _ := json.Marshal(q.Options)
					database.DB.Exec(`
						INSERT INTO quiz_questions (quiz_id, question, options, correct_answer, created_at)
						VALUES ($1, $2, $3, $4, NOW())
					`, quizID, q.Question, optionsJSON, q.Correct)
				}
			}
		}
	}

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"success":  true,
		"video_id": strconv.Itoa(videoID),
		"message":  "Video uploaded successfully",
	})
}
