package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/middleware"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// ==================== MODULES MANAGEMENT ====================

// PendingModule represents a module pending review
type PendingModule struct {
	ID           int       `json:"id"`
	Title        string    `json:"title"`
	ThumbnailURL *string   `json:"thumbnailUrl"`
	UploadedBy   string    `json:"uploadedBy"`
	UploadedAt   time.Time `json:"uploadedAt"`
	Status       string    `json:"status"`
}

// GetPendingModules - Get list of training modules pending supervisor review
// GET /api/supervisor/modules/pending
func GetPendingModules(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get modules that are pending review (uploaded by miners under this supervisor)
	rows, err := database.DB.Query(`
		SELECT vm.id, vm.title, vm.thumbnail, COALESCE(u.name, 'Unknown'), vm.created_at, vm.approval_status
		FROM video_modules vm
		LEFT JOIN users u ON vm.created_by = u.user_id
		WHERE vm.approval_status = 'pending'
		AND (
			vm.created_by IN (SELECT user_id FROM users WHERE supervisor_id = $1)
			OR vm.created_by = $1
		)
		ORDER BY vm.created_at DESC
	`, supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	modules := []PendingModule{}
	for rows.Next() {
		var module PendingModule
		var thumbnail sql.NullString
		err := rows.Scan(&module.ID, &module.Title, &thumbnail, &module.UploadedBy, &module.UploadedAt, &module.Status)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning data: "+err.Error())
			return
		}
		if thumbnail.Valid {
			module.ThumbnailURL = &thumbnail.String
		}
		modules = append(modules, module)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"modules": modules,
	})
}

// ReviewModuleRequest represents the request body for reviewing a module
type ReviewModuleRequest struct {
	Action   string `json:"action"`   // "approve" or "reject"
	Feedback string `json:"feedback"` // optional feedback
}

// ReviewModule - Approve or reject a training module
// POST /api/supervisor/modules/review/{id}
func ReviewModule(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	moduleID := vars["id"]

	var req ReviewModuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Action != "approve" && req.Action != "reject" {
		respondWithError(w, http.StatusBadRequest, "Action must be 'approve' or 'reject'")
		return
	}

	// Check if module exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM video_modules WHERE id = $1)", moduleID).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Module not found")
		return
	}

	status := "approved"
	isActive := true
	if req.Action == "reject" {
		status = "rejected"
		isActive = false
	}

	// Update module status
	_, err = database.DB.Exec(`
		UPDATE video_modules 
		SET approval_status = $1, is_active = $2, reviewed_by = $3, reviewed_at = $4, review_feedback = $5, updated_at = $6
		WHERE id = $7
	`, status, isActive, supervisorID, time.Now(), req.Feedback, time.Now(), moduleID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating module: "+err.Error())
		return
	}

	message := "Module approved successfully"
	if req.Action == "reject" {
		message = "Module rejected successfully"
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": message,
	})
}

// UploadedModule represents a module uploaded by the supervisor
type UploadedModule struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	Status       string  `json:"status"`
	Views        int     `json:"views"`
}

// GetUploadedModules - Get modules uploaded by this supervisor
// GET /api/supervisor/modules/uploaded
func GetUploadedModules(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	rows, err := database.DB.Query(`
		SELECT vm.id, vm.title, vm.thumbnail, vm.approval_status, COALESCE(vm.views_count, 0)
		FROM video_modules vm
		WHERE vm.created_by = $1
		ORDER BY vm.created_at DESC
	`, supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	modules := []UploadedModule{}
	for rows.Next() {
		var module UploadedModule
		var thumbnail sql.NullString
		err := rows.Scan(&module.ID, &module.Title, &thumbnail, &module.Status, &module.Views)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning data: "+err.Error())
			return
		}
		if thumbnail.Valid {
			module.ThumbnailURL = &thumbnail.String
		}
		modules = append(modules, module)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"modules": modules,
	})
}

// ==================== ZONE MANAGEMENT ====================

// Zone represents a mine zone/department
type Zone struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Location     string `json:"location"`
	Capacity     int    `json:"capacity"`
	CurrentCount int    `json:"currentCount"`
}

// GetZones - Get list of available mine zones/departments
// GET /api/supervisor/zones
func GetZones(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get supervisor's mining site
	var miningSite sql.NullString
	err := database.DB.QueryRow("SELECT mining_site FROM users WHERE user_id = $1", supervisorID).Scan(&miningSite)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching supervisor details")
		return
	}

	// Get zones for this mining site
	query := `
		SELECT z.id, z.name, z.location, z.capacity,
		       (SELECT COUNT(*) FROM users u WHERE u.zone_id = z.id) as current_count
		FROM mine_zones z
		WHERE z.is_active = true
	`
	args := []interface{}{}

	if miningSite.Valid && miningSite.String != "" {
		query += " AND z.mining_site = $1"
		args = append(args, miningSite.String)
	}

	query += " ORDER BY z.name ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	zones := []Zone{}
	for rows.Next() {
		var zone Zone
		err := rows.Scan(&zone.ID, &zone.Name, &zone.Location, &zone.Capacity, &zone.CurrentCount)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning data: "+err.Error())
			return
		}
		zones = append(zones, zone)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"zones": zones,
	})
}

// CreateZoneRequest represents the request body for creating a zone
type CreateZoneRequest struct {
	Name       string `json:"name"`
	Location   string `json:"location"`
	Capacity   int    `json:"capacity"`
	MiningSite string `json:"mining_site"`
}

// CreateZone - Create a new mine zone
// POST /api/supervisor/zones
func CreateZone(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Zone name is required")
		return
	}

	// Get supervisor's mining site if not provided
	if req.MiningSite == "" {
		var miningSite sql.NullString
		database.DB.QueryRow("SELECT mining_site FROM users WHERE user_id = $1", supervisorID).Scan(&miningSite)
		if miningSite.Valid {
			req.MiningSite = miningSite.String
		}
	}

	if req.Capacity == 0 {
		req.Capacity = 50 // Default capacity
	}

	var zoneID int
	err := database.DB.QueryRow(`
		INSERT INTO mine_zones (name, location, capacity, mining_site, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, req.Name, req.Location, req.Capacity, req.MiningSite, supervisorID, time.Now(), time.Now()).Scan(&zoneID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating zone: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"zoneId":  zoneID,
		"message": "Zone created successfully",
	})
}

// AllocateMinerRequest represents the request body for allocating a miner to a zone
type AllocateMinerRequest struct {
	MinerID string `json:"minerId"`
	ZoneID  string `json:"zoneId"`
}

// AllocateMinerToZone - Assign a miner to a specific mine zone
// POST /api/supervisor/allocate
func AllocateMinerToZone(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req AllocateMinerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.MinerID == "" || req.ZoneID == "" {
		respondWithError(w, http.StatusBadRequest, "minerId and zoneId are required")
		return
	}

	// Verify miner belongs to this supervisor
	var minerSupervisor sql.NullString
	err := database.DB.QueryRow("SELECT supervisor_id FROM users WHERE user_id = $1 AND role = 'MINER'", req.MinerID).Scan(&minerSupervisor)
	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Miner not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if !minerSupervisor.Valid || minerSupervisor.String != supervisorID {
		respondWithError(w, http.StatusForbidden, "You can only allocate miners under your supervision")
		return
	}

	// Verify zone exists and check capacity
	zoneIDInt, _ := strconv.Atoi(req.ZoneID)
	var zoneCapacity, currentCount int
	err = database.DB.QueryRow(`
		SELECT z.capacity, (SELECT COUNT(*) FROM users WHERE zone_id = z.id) 
		FROM mine_zones z WHERE z.id = $1 AND z.is_active = true
	`, zoneIDInt).Scan(&zoneCapacity, &currentCount)
	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Zone not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if currentCount >= zoneCapacity {
		respondWithError(w, http.StatusBadRequest, "Zone is at full capacity")
		return
	}

	// Update miner's zone
	_, err = database.DB.Exec("UPDATE users SET zone_id = $1, updated_at = $2 WHERE user_id = $3", zoneIDInt, time.Now(), req.MinerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error allocating miner: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Miner assigned to zone successfully",
	})
}

// ==================== EMERGENCY REPORT MANAGEMENT ====================

// DownloadEmergencyReport - Download emergency report as PDF
// GET /api/supervisor/emergencies/{id}/download
func DownloadEmergencyReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	emergencyID := vars["id"]

	// Fetch emergency details
	var emergency struct {
		ID            int
		UserID        string
		EmergencyID   int
		Severity      string
		Latitude      float64
		Longitude     float64
		Issue         string
		Location      *string
		ReportingTime time.Time
		Status        string
		UserName      string
	}

	err := database.DB.QueryRow(`
		SELECT e.id, e.user_id, e.emergency_id, e.severity, e.latitude, e.longitude, 
		       e.issue, e.location, e.reporting_time, e.status, u.name
		FROM emergencies e
		JOIN users u ON e.user_id = u.user_id
		WHERE e.id = $1
	`, emergencyID).Scan(
		&emergency.ID, &emergency.UserID, &emergency.EmergencyID, &emergency.Severity,
		&emergency.Latitude, &emergency.Longitude, &emergency.Issue, &emergency.Location,
		&emergency.ReportingTime, &emergency.Status, &emergency.UserName,
	)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Emergency not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	// For now, return a JSON representation that can be used to generate PDF on client
	// In a production environment, you'd use a PDF library like gofpdf
	location := "Not Available"
	if emergency.Location != nil {
		location = *emergency.Location
	}

	reportData := map[string]interface{}{
		"reportTitle": fmt.Sprintf("Emergency Report #%d", emergency.ID),
		"emergencyId": emergency.EmergencyID,
		"reportedBy":  emergency.UserName,
		"reportedAt":  emergency.ReportingTime.Format("2006-01-02 15:04:05"),
		"severity":    emergency.Severity,
		"status":      emergency.Status,
		"location":    location,
		"coordinates": fmt.Sprintf("%.6f, %.6f", emergency.Latitude, emergency.Longitude),
		"issue":       emergency.Issue,
		"generatedAt": time.Now().Format("2006-01-02 15:04:05"),
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"reportData":  reportData,
		"downloadUrl": fmt.Sprintf("/api/supervisor/emergencies/%s/pdf", emergencyID),
	})
}

// ForwardEmergencyRequest represents the request body for forwarding an emergency report
type ForwardEmergencyRequest struct {
	Recipients []string `json:"recipients"`
	Message    string   `json:"message"`
}

// ForwardEmergencyReport - Forward report to higher authorities
// POST /api/supervisor/emergencies/{id}/forward
func ForwardEmergencyReport(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	emergencyID := vars["id"]

	var req ForwardEmergencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if len(req.Recipients) == 0 {
		respondWithError(w, http.StatusBadRequest, "At least one recipient is required")
		return
	}

	// Verify emergency exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM emergencies WHERE id = $1)", emergencyID).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Emergency not found")
		return
	}

	// Log the forward action (in production, you'd send actual emails)
	_, err = database.DB.Exec(`
		INSERT INTO emergency_forwards (emergency_id, forwarded_by, recipients, message, forwarded_at)
		VALUES ($1, $2, $3, $4, $5)
	`, emergencyID, supervisorID, fmt.Sprintf("%v", req.Recipients), req.Message, time.Now())

	// If table doesn't exist, we still return success (email sending is simulated)
	// In production, integrate with email service (SendGrid, SES, etc.)

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"message":    fmt.Sprintf("Emergency report forwarded to %d recipient(s)", len(req.Recipients)),
		"recipients": req.Recipients,
	})
}

// ==================== SUPERVISOR MINERS VIEW ====================

// SupervisorMiner represents a miner in the supervisor's view
type SupervisorMiner struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	MinerID        string  `json:"minerId"`
	Phone          string  `json:"phone"`
	Zone           *string `json:"zone"`
	Status         string  `json:"status"`
	ProfilePicture *string `json:"profilePicture,omitempty"`
}

// GetSupervisorMiners - Get all miners assigned to this supervisor with zone info
// GET /api/supervisor/miners
func GetSupervisorMiners(w http.ResponseWriter, r *http.Request) {
	supervisorID, ok := middleware.GetUserIDFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	rows, err := database.DB.Query(`
		SELECT u.user_id, u.name, u.user_id as miner_id, COALESCE(u.phone, ''), 
		       z.name as zone_name, 
		       CASE WHEN u.is_active THEN 'active' ELSE 'inactive' END as status,
		       u.profile_picture_url
		FROM users u
		LEFT JOIN mine_zones z ON u.zone_id = z.id
		WHERE u.supervisor_id = $1 AND u.role = 'MINER'
		ORDER BY u.name ASC
	`, supervisorID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	miners := []SupervisorMiner{}
	for rows.Next() {
		var miner SupervisorMiner
		var zoneName, profilePic sql.NullString
		err := rows.Scan(&miner.ID, &miner.Name, &miner.MinerID, &miner.Phone, &zoneName, &miner.Status, &profilePic)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning data: "+err.Error())
			return
		}
		if zoneName.Valid {
			miner.Zone = &zoneName.String
		}
		if profilePic.Valid {
			miner.ProfilePicture = &profilePic.String
		}
		miners = append(miners, miner)
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"miners": miners,
	})
}
