package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/middleware"
	"MineSafeBackend/models"
	"net/http"
	"time"
)

// GetLearningStreaks - Get learning streaks for all miners under a supervisor
func GetLearningStreaks(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get all miners under this supervisor with their streaks
	rows, err := database.DB.Query(`
		SELECT 
			u.user_id,
			u.name,
			COALESCE(COUNT(DISTINCT DATE(mc.completed_at)), 0) as current_streak,
			COALESCE(MAX(mc.completed_at), u.created_at) as last_completed,
			COALESCE(COUNT(mc.id), 0) as total_modules
		FROM users u
		LEFT JOIN module_completions mc ON u.user_id = mc.miner_id 
			AND mc.completed_at >= NOW() - INTERVAL '30 days'
		WHERE u.supervisor_id = $1 AND u.role = 'MINER'
		GROUP BY u.user_id, u.name, u.created_at
		ORDER BY current_streak DESC, u.name
	`, supervisorID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	streaks := []models.LearningStreak{}
	for rows.Next() {
		var streak models.LearningStreak
		err := rows.Scan(&streak.MinerID, &streak.MinerName, &streak.CurrentStreak, 
			&streak.LastCompleted, &streak.TotalModules)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning streak data")
			return
		}
		streaks = append(streaks, streak)
	}

	respondWithJSON(w, http.StatusOK, streaks)
}

// GetMinerStreak - Get learning streak for a specific miner
func GetMinerStreak(w http.ResponseWriter, r *http.Request) {
	minerID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var streak models.LearningStreak
	err := database.DB.QueryRow(`
		SELECT 
			u.user_id,
			u.name,
			COALESCE(COUNT(DISTINCT DATE(mc.completed_at)), 0) as current_streak,
			COALESCE(MAX(mc.completed_at), u.created_at) as last_completed,
			COALESCE(COUNT(mc.id), 0) as total_modules
		FROM users u
		LEFT JOIN module_completions mc ON u.user_id = mc.miner_id 
			AND mc.completed_at >= NOW() - INTERVAL '30 days'
		WHERE u.user_id = $1
		GROUP BY u.user_id, u.name, u.created_at
	`, minerID).Scan(&streak.MinerID, &streak.MinerName, &streak.CurrentStreak, 
		&streak.LastCompleted, &streak.TotalModules)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, streak)
}

// GetMinerCompletions - Get all module completions for a miner
func GetMinerCompletions(w http.ResponseWriter, r *http.Request) {
	minerID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	rows, err := database.DB.Query(`
		SELECT 
			mc.id,
			mc.miner_id,
			mc.video_id,
			mc.completed_at,
			mc.score,
			mc.total_questions,
			vm.title as video_title
		FROM module_completions mc
		JOIN video_modules vm ON mc.video_id = vm.id
		WHERE mc.miner_id = $1
		ORDER BY mc.completed_at DESC
		LIMIT 50
	`, minerID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	completions := []map[string]interface{}{}
	for rows.Next() {
		var completion struct {
			ID             int       `json:"id"`
			MinerID        string    `json:"miner_id"`
			VideoID        int       `json:"video_id"`
			CompletedAt    time.Time `json:"completed_at"`
			Score          int       `json:"score"`
			TotalQuestions int       `json:"total_questions"`
			VideoTitle     string    `json:"video_title"`
		}
		err := rows.Scan(&completion.ID, &completion.MinerID, &completion.VideoID,
			&completion.CompletedAt, &completion.Score, &completion.TotalQuestions, &completion.VideoTitle)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning completion data")
			return
		}
		
		completionMap := map[string]interface{}{
			"id":              completion.ID,
			"miner_id":        completion.MinerID,
			"video_id":        completion.VideoID,
			"completed_at":    completion.CompletedAt,
			"score":           completion.Score,
			"total_questions": completion.TotalQuestions,
			"video_title":     completion.VideoTitle,
			"percentage":      float64(completion.Score) / float64(completion.TotalQuestions) * 100,
		}
		completions = append(completions, completionMap)
	}

	respondWithJSON(w, http.StatusOK, completions)
}

// GetDashboardStats - Get dashboard statistics for supervisor
func GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	stats := make(map[string]interface{})

	// Total miners
	var totalMiners int
	database.DB.QueryRow(
		"SELECT COUNT(*) FROM users WHERE supervisor_id = $1 AND role = 'MINER'",
		supervisorID,
	).Scan(&totalMiners)
	stats["total_miners"] = totalMiners

	// Active miners (completed a module in last 7 days)
	var activeMiners int
	database.DB.QueryRow(`
		SELECT COUNT(DISTINCT miner_id) 
		FROM module_completions mc
		JOIN users u ON mc.miner_id = u.user_id
		WHERE u.supervisor_id = $1 AND mc.completed_at >= NOW() - INTERVAL '7 days'
	`, supervisorID).Scan(&activeMiners)
	stats["active_miners"] = activeMiners

	// Total video modules
	var totalModules int
	database.DB.QueryRow("SELECT COUNT(*) FROM video_modules WHERE is_active = true").Scan(&totalModules)
	stats["total_modules"] = totalModules

	// Total completions this month
	var monthlyCompletions int
	database.DB.QueryRow(`
		SELECT COUNT(*) 
		FROM module_completions mc
		JOIN users u ON mc.miner_id = u.user_id
		WHERE u.supervisor_id = $1 
		AND mc.completed_at >= DATE_TRUNC('month', NOW())
	`, supervisorID).Scan(&monthlyCompletions)
	stats["monthly_completions"] = monthlyCompletions

	// Average score
	var avgScore float64
	database.DB.QueryRow(`
		SELECT COALESCE(AVG(CAST(mc.score AS FLOAT) / CAST(mc.total_questions AS FLOAT) * 100), 0)
		FROM module_completions mc
		JOIN users u ON mc.miner_id = u.user_id
		WHERE u.supervisor_id = $1
	`, supervisorID).Scan(&avgScore)
	stats["average_score"] = avgScore

	// Today's star video completion count
	var todayCompletions int
	database.DB.QueryRow(`
		SELECT COUNT(DISTINCT mc.miner_id)
		FROM module_completions mc
		JOIN star_videos sv ON mc.video_id = sv.video_id
		JOIN users u ON mc.miner_id = u.user_id
		WHERE u.supervisor_id = $1 
		AND sv.supervisor_id = $1
		AND sv.set_date = CURRENT_DATE
		AND sv.is_active = true
		AND mc.completed_at::date = CURRENT_DATE
	`, supervisorID).Scan(&todayCompletions)
	stats["today_completions"] = todayCompletions

	respondWithJSON(w, http.StatusOK, stats)
}
