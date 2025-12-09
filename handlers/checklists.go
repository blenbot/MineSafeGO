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

// ==================== CALENDAR & STREAK ROUTES (User Protected) ====================

// GetQuizCalendarAndStreak - Get quiz attempt dates and streak for calendar display
// GET /api/app/quiz-calendar
// Response: CalendarStreakResponse with attempt_dates array and streak info
func GetQuizCalendarAndStreak(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get user name
	var userName string
	err := database.DB.QueryRow("SELECT name FROM users WHERE user_id = $1", userID).Scan(&userName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching user")
		return
	}

	// Get all distinct dates when user attempted quizzes
	rows, err := database.DB.Query(`
		SELECT DISTINCT DATE(completed_at) as attempt_date
		FROM module_completions
		WHERE miner_id = $1
		ORDER BY attempt_date DESC
	`, userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	attemptDates := []string{}
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning date")
			return
		}
		attemptDates = append(attemptDates, date.Format("2006-01-02"))
	}

	// Calculate current streak (consecutive days ending today or yesterday)
	currentStreak := calculateCurrentStreak(attemptDates)
	longestStreak := calculateLongestStreak(attemptDates)

	response := models.CalendarStreakResponse{
		UserID:        userID,
		UserName:      userName,
		CurrentStreak: currentStreak,
		LongestStreak: longestStreak,
		TotalDays:     len(attemptDates),
		AttemptDates:  attemptDates,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// calculateCurrentStreak calculates consecutive days ending today or yesterday
func calculateCurrentStreak(dates []string) int {
	if len(dates) == 0 {
		return 0
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Check if most recent date is today or yesterday
	if dates[0] != today && dates[0] != yesterday {
		return 0
	}

	streak := 1
	for i := 1; i < len(dates); i++ {
		currentDate, _ := time.Parse("2006-01-02", dates[i-1])
		prevDate, _ := time.Parse("2006-01-02", dates[i])

		// Check if dates are consecutive
		if currentDate.AddDate(0, 0, -1).Format("2006-01-02") == prevDate.Format("2006-01-02") {
			streak++
		} else {
			break
		}
	}

	return streak
}

// calculateLongestStreak calculates the longest consecutive streak
func calculateLongestStreak(dates []string) int {
	if len(dates) == 0 {
		return 0
	}

	longest := 1
	current := 1

	for i := 1; i < len(dates); i++ {
		currentDate, _ := time.Parse("2006-01-02", dates[i-1])
		prevDate, _ := time.Parse("2006-01-02", dates[i])

		if currentDate.AddDate(0, 0, -1).Format("2006-01-02") == prevDate.Format("2006-01-02") {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 1
		}
	}

	return longest
}

// ==================== PRE-START CHECKLIST ROUTES ====================

// CreatePreStartChecklistItem - Supervisor creates a new pre-start checklist item
// POST /api/checklists/pre-start
func CreatePreStartChecklistItem(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var item models.ChecklistItemCreate
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if item.Title == "" {
		respondWithError(w, http.StatusBadRequest, "Title is required")
		return
	}

	var itemID int
	err := database.DB.QueryRow(`
		INSERT INTO pre_start_checklist (supervisor_id, title, description, is_default, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, false, true, NOW(), NOW())
		RETURNING id
	`, supervisorID, item.Title, item.Description).Scan(&itemID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating checklist item: "+err.Error())
		return
	}

	var newItem models.PreStartChecklistItem
	err = database.DB.QueryRow(`
		SELECT id, supervisor_id, title, description, is_default, is_active, created_at, updated_at
		FROM pre_start_checklist WHERE id = $1
	`, itemID).Scan(&newItem.ID, &newItem.SupervisorID, &newItem.Title, &newItem.Description,
		&newItem.IsDefault, &newItem.IsActive, &newItem.CreatedAt, &newItem.UpdatedAt)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching created item")
		return
	}

	respondWithJSON(w, http.StatusCreated, newItem)
}

// GetPreStartChecklistItems - Supervisor gets all pre-start checklist items
// GET /api/checklists/pre-start
func GetPreStartChecklistItems(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, supervisor_id, title, description, is_default, is_active, created_at, updated_at
		FROM pre_start_checklist
		WHERE (supervisor_id = $1 OR is_default = true) AND is_active = true
		ORDER BY is_default DESC, created_at ASC
	`, supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	items := []models.PreStartChecklistItem{}
	for rows.Next() {
		var item models.PreStartChecklistItem
		err := rows.Scan(&item.ID, &item.SupervisorID, &item.Title, &item.Description,
			&item.IsDefault, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning item")
			return
		}
		items = append(items, item)
	}

	respondWithJSON(w, http.StatusOK, items)
}

// DeletePreStartChecklistItem - Supervisor deletes a pre-start checklist item
// DELETE /api/checklists/pre-start/{id}
func DeletePreStartChecklistItem(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	itemID := vars["id"]

	// Can only delete non-default items created by this supervisor
	result, err := database.DB.Exec(`
		UPDATE pre_start_checklist SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND supervisor_id = $2 AND is_default = false
	`, itemID, supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Item not found or cannot be deleted")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Item deleted successfully"})
}

// UpdatePreStartChecklistCompletion - Supervisor marks item as completed/not completed
// PUT /api/checklists/pre-start/complete
func UpdatePreStartChecklistCompletion(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var update models.ChecklistCompletionUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	today := time.Now().Format("2006-01-02")

	// Upsert completion record
	_, err := database.DB.Exec(`
		INSERT INTO pre_start_checklist_completions (user_id, item_id, is_completed, completed_at, date)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT (user_id, item_id, date)
		DO UPDATE SET is_completed = $3, completed_at = NOW()
	`, supervisorID, update.ItemID, update.IsCompleted, today)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating completion: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Completion updated successfully"})
}

// GetPreStartChecklistForApp - User (Miner) gets pre-start checklist with status
// GET /api/app/checklists/pre-start
func GetPreStartChecklistForApp(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get user's supervisor ID
	var supervisorID *string
	err := database.DB.QueryRow("SELECT supervisor_id FROM users WHERE user_id = $1", userID).Scan(&supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching user")
		return
	}

	if supervisorID == nil {
		respondWithError(w, http.StatusBadRequest, "User is not assigned to a supervisor")
		return
	}

	today := time.Now().Format("2006-01-02")

	rows, err := database.DB.Query(`
		SELECT 
			p.id, p.title, p.description,
			COALESCE(c.is_completed, false) as is_completed,
			c.completed_at
		FROM pre_start_checklist p
		LEFT JOIN pre_start_checklist_completions c 
			ON p.id = c.item_id AND c.user_id = $1 AND c.date = $3
		WHERE (p.supervisor_id = $2 OR p.is_default = true) AND p.is_active = true
		ORDER BY p.is_default DESC, p.created_at ASC
	`, userID, *supervisorID, today)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	items := []models.ChecklistItemWithStatus{}
	for rows.Next() {
		var item models.ChecklistItemWithStatus
		var completedAt sql.NullTime
		err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.IsCompleted, &completedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning item")
			return
		}
		if completedAt.Valid {
			item.CompletedAt = &completedAt.Time
		}
		items = append(items, item)
	}

	respondWithJSON(w, http.StatusOK, items)
}

// UpdatePreStartChecklistForApp - User (Miner) marks item as completed
// PUT /api/app/checklists/pre-start/complete
func UpdatePreStartChecklistForApp(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var update models.ChecklistCompletionUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	today := time.Now().Format("2006-01-02")

	_, err := database.DB.Exec(`
		INSERT INTO pre_start_checklist_completions (user_id, item_id, is_completed, completed_at, date)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT (user_id, item_id, date)
		DO UPDATE SET is_completed = $3, completed_at = NOW()
	`, userID, update.ItemID, update.IsCompleted, today)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating completion: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Completion updated successfully"})
}

// ==================== PPE CHECKLIST ROUTES ====================

// CreatePPEChecklistItem - Supervisor creates a new PPE checklist item
// POST /api/checklists/ppe
func CreatePPEChecklistItem(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var item models.ChecklistItemCreate
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if item.Title == "" {
		respondWithError(w, http.StatusBadRequest, "Title is required")
		return
	}

	var itemID int
	err := database.DB.QueryRow(`
		INSERT INTO ppe_checklist (supervisor_id, title, description, is_default, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, false, true, NOW(), NOW())
		RETURNING id
	`, supervisorID, item.Title, item.Description).Scan(&itemID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating checklist item: "+err.Error())
		return
	}

	var newItem models.PPEChecklistItem
	err = database.DB.QueryRow(`
		SELECT id, supervisor_id, title, description, is_default, is_active, created_at, updated_at
		FROM ppe_checklist WHERE id = $1
	`, itemID).Scan(&newItem.ID, &newItem.SupervisorID, &newItem.Title, &newItem.Description,
		&newItem.IsDefault, &newItem.IsActive, &newItem.CreatedAt, &newItem.UpdatedAt)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching created item")
		return
	}

	respondWithJSON(w, http.StatusCreated, newItem)
}

// GetPPEChecklistItems - Supervisor gets all PPE checklist items
// GET /api/checklists/ppe
func GetPPEChecklistItems(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	rows, err := database.DB.Query(`
		SELECT id, supervisor_id, title, description, is_default, is_active, created_at, updated_at
		FROM ppe_checklist
		WHERE (supervisor_id = $1 OR is_default = true) AND is_active = true
		ORDER BY is_default DESC, created_at ASC
	`, supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	items := []models.PPEChecklistItem{}
	for rows.Next() {
		var item models.PPEChecklistItem
		err := rows.Scan(&item.ID, &item.SupervisorID, &item.Title, &item.Description,
			&item.IsDefault, &item.IsActive, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning item")
			return
		}
		items = append(items, item)
	}

	respondWithJSON(w, http.StatusOK, items)
}

// DeletePPEChecklistItem - Supervisor deletes a PPE checklist item
// DELETE /api/checklists/ppe/{id}
func DeletePPEChecklistItem(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	itemID := vars["id"]

	result, err := database.DB.Exec(`
		UPDATE ppe_checklist SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND supervisor_id = $2 AND is_default = false
	`, itemID, supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Item not found or cannot be deleted")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Item deleted successfully"})
}

// UpdatePPEChecklistCompletion - Supervisor marks PPE item as completed/not completed
// PUT /api/checklists/ppe/complete
func UpdatePPEChecklistCompletion(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var update models.ChecklistCompletionUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	today := time.Now().Format("2006-01-02")

	_, err := database.DB.Exec(`
		INSERT INTO ppe_checklist_completions (user_id, item_id, is_completed, completed_at, date)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT (user_id, item_id, date)
		DO UPDATE SET is_completed = $3, completed_at = NOW()
	`, supervisorID, update.ItemID, update.IsCompleted, today)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating completion: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Completion updated successfully"})
}

// GetPPEChecklistForApp - User (Miner) gets PPE checklist with status
// GET /api/app/checklists/ppe
func GetPPEChecklistForApp(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get user's supervisor ID
	var supervisorID *string
	err := database.DB.QueryRow("SELECT supervisor_id FROM users WHERE user_id = $1", userID).Scan(&supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching user")
		return
	}

	if supervisorID == nil {
		respondWithError(w, http.StatusBadRequest, "User is not assigned to a supervisor")
		return
	}

	today := time.Now().Format("2006-01-02")

	rows, err := database.DB.Query(`
		SELECT 
			p.id, p.title, p.description,
			COALESCE(c.is_completed, false) as is_completed,
			c.completed_at
		FROM ppe_checklist p
		LEFT JOIN ppe_checklist_completions c 
			ON p.id = c.item_id AND c.user_id = $1 AND c.date = $3
		WHERE (p.supervisor_id = $2 OR p.is_default = true) AND p.is_active = true
		ORDER BY p.is_default DESC, p.created_at ASC
	`, userID, *supervisorID, today)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	items := []models.ChecklistItemWithStatus{}
	for rows.Next() {
		var item models.ChecklistItemWithStatus
		var completedAt sql.NullTime
		err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.IsCompleted, &completedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning item")
			return
		}
		if completedAt.Valid {
			item.CompletedAt = &completedAt.Time
		}
		items = append(items, item)
	}

	respondWithJSON(w, http.StatusOK, items)
}

// UpdatePPEChecklistForApp - User (Miner) marks PPE item as completed
// PUT /api/app/checklists/ppe/complete
func UpdatePPEChecklistForApp(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var update models.ChecklistCompletionUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	today := time.Now().Format("2006-01-02")

	_, err := database.DB.Exec(`
		INSERT INTO ppe_checklist_completions (user_id, item_id, is_completed, completed_at, date)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT (user_id, item_id, date)
		DO UPDATE SET is_completed = $3, completed_at = NOW()
	`, userID, update.ItemID, update.IsCompleted, today)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating completion: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Completion updated successfully"})
}
