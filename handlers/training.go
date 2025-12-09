package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/middleware"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
)

// ==================== QUIZ RESPONSES ====================

type QuizQuestion struct {
	ID       int      `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	Correct  int      `json:"correct"`
	Tags     []string `json:"tags,omitempty"`
}

type QuizByTitleResponse struct {
	Title        string         `json:"title"`
	NumQuestions int            `json:"num_questions"`
	VideoName    string         `json:"video_name"`
	Questions    []QuizQuestion `json:"questions"`
}

type QuizListItem struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	VideoTitle   string   `json:"video_title"`
	NumQuestions int      `json:"num_questions"`
	Tags         []string `json:"tags"`
	Completed    bool     `json:"completed"`
	BestScore    *int     `json:"best_score"`
}

type QuizListResponse struct {
	Quizzes []QuizListItem `json:"quizzes"`
}

// ==================== QUIZ ENDPOINTS ====================

// GetQuizByTitle - GET /api/training/quiz?title=Safety%20Helmet%20Usage
func GetQuizByTitle(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	title := r.URL.Query().Get("title")
	if title == "" {
		respondWithError(w, http.StatusBadRequest, "Title parameter is required")
		return
	}

	// First try to find a quiz by video title (case-insensitive partial match)
	var quizID int
	var quizTitle, videoTitle string
	var tagsJSON []byte

	// Try to find quiz linked to video with matching title
	err := database.DB.QueryRow(`
		SELECT q.id, q.title, vm.title as video_title, COALESCE(q.tags, '[]'::jsonb) as tags
		FROM quizzes q
		JOIN video_modules vm ON q.video_id = vm.id
		WHERE LOWER(vm.title) LIKE LOWER($1)
		LIMIT 1
	`, "%"+title+"%").Scan(&quizID, &quizTitle, &videoTitle, &tagsJSON)

	if err == sql.ErrNoRows {
		// Try to find questions directly linked to video_modules (legacy support)
		var videoID int
		err = database.DB.QueryRow(`
			SELECT id, title FROM video_modules 
			WHERE LOWER(title) LIKE LOWER($1) AND is_active = true
			LIMIT 1
		`, "%"+title+"%").Scan(&videoID, &videoTitle)

		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "No quiz found for the given video title")
			return
		}
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Database error")
			return
		}

		// Get questions from the questions table (legacy)
		rows, err := database.DB.Query(`
			SELECT id, question, options, answer
			FROM questions
			WHERE video_id = $1
		`, videoID)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Database error")
			return
		}
		defer rows.Close()

		questions := []QuizQuestion{}
		for rows.Next() {
			var q QuizQuestion
			var optionsJSON []byte
			err := rows.Scan(&q.ID, &q.Question, &optionsJSON, &q.Correct)
			if err != nil {
				continue
			}
			json.Unmarshal(optionsJSON, &q.Options)
			if q.Options == nil {
				q.Options = []string{}
			}
			q.Tags = []string{}
			questions = append(questions, q)
		}

		if len(questions) == 0 {
			respondWithError(w, http.StatusNotFound, "No questions found for this video")
			return
		}

		respondWithJSON(w, http.StatusOK, QuizByTitleResponse{
			Title:        "Safety Quiz: " + videoTitle,
			NumQuestions: len(questions),
			VideoName:    videoTitle,
			Questions:    questions,
		})
		return
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	// Get quiz questions
	rows, err := database.DB.Query(`
		SELECT id, question, options, correct_answer, COALESCE(tags, '[]'::jsonb) as tags
		FROM quiz_questions
		WHERE quiz_id = $1
	`, quizID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	questions := []QuizQuestion{}
	for rows.Next() {
		var q QuizQuestion
		var optionsJSON, qTagsJSON []byte
		err := rows.Scan(&q.ID, &q.Question, &optionsJSON, &q.Correct, &qTagsJSON)
		if err != nil {
			continue
		}
		json.Unmarshal(optionsJSON, &q.Options)
		json.Unmarshal(qTagsJSON, &q.Tags)
		if q.Options == nil {
			q.Options = []string{}
		}
		if q.Tags == nil {
			q.Tags = []string{}
		}
		questions = append(questions, q)
	}

	respondWithJSON(w, http.StatusOK, QuizByTitleResponse{
		Title:        quizTitle,
		NumQuestions: len(questions),
		VideoName:    videoTitle,
		Questions:    questions,
	})
}

// GetQuizList - GET /api/training/quizzes
func GetQuizList(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get all quizzes with completion status
	rows, err := database.DB.Query(`
		SELECT 
			q.id, q.title, vm.title as video_title, 
			COALESCE(q.tags, '[]'::jsonb) as tags,
			(SELECT COUNT(*) FROM quiz_questions WHERE quiz_id = q.id) as num_questions,
			EXISTS(SELECT 1 FROM quiz_completions WHERE quiz_id = q.id AND user_id = $1) as completed,
			(SELECT MAX(score) FROM quiz_completions WHERE quiz_id = q.id AND user_id = $1) as best_score
		FROM quizzes q
		JOIN video_modules vm ON q.video_id = vm.id
		WHERE vm.is_active = true
		ORDER BY q.created_at DESC
	`, userID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	quizzes := []QuizListItem{}
	for rows.Next() {
		var quiz QuizListItem
		var tagsJSON []byte
		var idInt int
		var bestScore sql.NullInt64

		err := rows.Scan(&idInt, &quiz.Title, &quiz.VideoTitle, &tagsJSON, &quiz.NumQuestions, &quiz.Completed, &bestScore)
		if err != nil {
			continue
		}

		quiz.ID = strconv.Itoa(idInt)
		json.Unmarshal(tagsJSON, &quiz.Tags)
		if quiz.Tags == nil {
			quiz.Tags = []string{}
		}

		if bestScore.Valid {
			score := int(bestScore.Int64)
			quiz.BestScore = &score
		}

		quizzes = append(quizzes, quiz)
	}

	// Also include videos with legacy questions (from questions table)
	legacyRows, err := database.DB.Query(`
		SELECT DISTINCT 
			vm.id, vm.title,
			COALESCE(vm.tags, '[]'::jsonb) as tags,
			(SELECT COUNT(*) FROM questions WHERE video_id = vm.id) as num_questions,
			EXISTS(SELECT 1 FROM module_completions WHERE video_id = vm.id AND miner_id = $1) as completed,
			(SELECT MAX(score) FROM module_completions WHERE video_id = vm.id AND miner_id = $1) as best_score
		FROM video_modules vm
		WHERE vm.is_active = true
		AND EXISTS(SELECT 1 FROM questions WHERE video_id = vm.id)
		AND NOT EXISTS(SELECT 1 FROM quizzes WHERE video_id = vm.id)
		ORDER BY vm.created_at DESC
	`, userID)

	if err == nil {
		defer legacyRows.Close()
		for legacyRows.Next() {
			var quiz QuizListItem
			var tagsJSON []byte
			var idInt int
			var bestScore sql.NullInt64

			err := legacyRows.Scan(&idInt, &quiz.VideoTitle, &tagsJSON, &quiz.NumQuestions, &quiz.Completed, &bestScore)
			if err != nil {
				continue
			}

			quiz.ID = "legacy-" + strconv.Itoa(idInt)
			quiz.Title = "Quiz: " + quiz.VideoTitle
			json.Unmarshal(tagsJSON, &quiz.Tags)
			if quiz.Tags == nil {
				quiz.Tags = []string{}
			}

			if bestScore.Valid {
				score := int(bestScore.Int64)
				quiz.BestScore = &score
			}

			quizzes = append(quizzes, quiz)
		}
	}

	respondWithJSON(w, http.StatusOK, QuizListResponse{
		Quizzes: quizzes,
	})
}

// ==================== VIDEO MODULES WITH QUIZZES ====================

type VideoModuleItem struct {
	Row          int      `json:"row"`
	VideoID      int      `json:"video_id"`
	VideoTitle   string   `json:"video_title"`
	VideoURL     string   `json:"video_url"`
	VideoTags    []string `json:"video_tags"`
	HasQuiz      bool     `json:"has_quiz"`
	QuizID       *int     `json:"quiz_id,omitempty"`
	QuizTitle    string   `json:"quiz_title,omitempty"`
	QuizTags     []string `json:"quiz_tags,omitempty"`
	NumQuestions int      `json:"num_questions"`
}

type VideoModulesResponse struct {
	Modules []VideoModuleItem `json:"modules"`
	Total   int               `json:"total"`
}

// GetVideoModulesWithQuizzes - GET /api/training/modules - Returns all videos with their quiz info in numbered rows
func GetVideoModulesWithQuizzes(w http.ResponseWriter, r *http.Request) {
	_, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get all video modules with their associated quiz info
	rows, err := database.DB.Query(`
		SELECT 
			vm.id as video_id,
			vm.title as video_title,
			vm.video_url,
			COALESCE(vm.tags, '[]'::jsonb) as video_tags,
			q.id as quiz_id,
			q.title as quiz_title,
			COALESCE(q.tags, '[]'::jsonb) as quiz_tags,
			(SELECT COUNT(*) FROM quiz_questions WHERE quiz_id = q.id) as num_questions
		FROM video_modules vm
		LEFT JOIN quizzes q ON q.video_id = vm.id
		WHERE vm.is_active = true
		ORDER BY vm.id ASC
	`)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	modules := []VideoModuleItem{}
	rowNum := 1
	for rows.Next() {
		var module VideoModuleItem
		var videoTagsJSON, quizTagsJSON []byte
		var quizID sql.NullInt64
		var quizTitle sql.NullString

		err := rows.Scan(
			&module.VideoID,
			&module.VideoTitle,
			&module.VideoURL,
			&videoTagsJSON,
			&quizID,
			&quizTitle,
			&quizTagsJSON,
			&module.NumQuestions,
		)
		if err != nil {
			continue
		}

		module.Row = rowNum
		rowNum++

		json.Unmarshal(videoTagsJSON, &module.VideoTags)
		if module.VideoTags == nil {
			module.VideoTags = []string{}
		}

		if quizID.Valid {
			module.HasQuiz = true
			qID := int(quizID.Int64)
			module.QuizID = &qID
			module.QuizTitle = quizTitle.String
			json.Unmarshal(quizTagsJSON, &module.QuizTags)
			if module.QuizTags == nil {
				module.QuizTags = []string{}
			}
		} else {
			module.HasQuiz = false
			module.QuizTags = []string{}
		}

		modules = append(modules, module)
	}

	respondWithJSON(w, http.StatusOK, VideoModulesResponse{
		Modules: modules,
		Total:   len(modules),
	})
}
