package handlers

import (
	"MineSafeBackend/database"
	"MineSafeBackend/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
)

// LocationIQ response structure
type LocationIQResponse struct {
	DisplayName string `json:"display_name"`
	Address     struct {
		Road        string `json:"road"`
		Village     string `json:"village"`
		County      string `json:"county"`
		State       string `json:"state"`
		Postcode    string `json:"postcode"`
		Country     string `json:"country"`
	} `json:"address"`
}

// CreateEmergency - Create a new emergency report
func CreateEmergency(w http.ResponseWriter, r *http.Request) {
	var emergencyData models.EmergencyCreate
	if err := json.NewDecoder(r.Body).Decode(&emergencyData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate required fields
	if emergencyData.UserID == "" {
		respondWithError(w, http.StatusBadRequest, "UserID is required")
		return
	}

	// Check if emergency already exists (duplicate detection)
	var existingID int
	err := database.DB.QueryRow(
		"SELECT id FROM emergencies WHERE user_id = $1 AND emergency_id = $2",
		emergencyData.UserID, emergencyData.EmergencyID,
	).Scan(&existingID)

	if err == nil {
		// Emergency already exists, return existing record
		var emergency models.Emergency
		err = database.DB.QueryRow(
			`SELECT id, user_id, emergency_id, severity, latitude, longitude, issue, 
			        media_status, media_url, location, incident_time, reporting_time, status, resolution_time
			 FROM emergencies WHERE id = $1`,
			existingID,
		).Scan(&emergency.ID, &emergency.UserID, &emergency.EmergencyID, &emergency.Severity,
			&emergency.Lat, &emergency.Lon, &emergency.Issue, &emergency.MediaStatus,
			&emergency.MediaURL, &emergency.Location, &emergency.IncidentTime,
			&emergency.IncidentReportingTime, &emergency.Status, &emergency.ResolutionTime)

		if err == nil {
			respondWithJSON(w, http.StatusOK, map[string]interface{}{
				"message":   "Emergency already exists",
				"emergency": emergency,
				"duplicate": true,
			})
			return
		}
	}

	// Set default media status if not provided
	if emergencyData.MediaStatus == "" {
		emergencyData.MediaStatus = models.StatusNotApplicable
	}

	// Reverse geocode location if coordinates are provided
	var location *string
	if emergencyData.Latitude != 0 && emergencyData.Longitude != 0 {
		locationStr, err := reverseGeocode(emergencyData.Latitude, emergencyData.Longitude)
		if err == nil {
			location = &locationStr
		}
	}

	// Create emergency
	emergency, err := models.NewEmergency(
		emergencyData.UserID,
		emergencyData.EmergencyID,
		emergencyData.Severity,
		emergencyData.Latitude,
		emergencyData.Longitude,
		emergencyData.Issue,
		emergencyData.MediaStatus,
		nil, // MediaURL will be updated later if media is uploaded
		nil, // IncidentTime can be set by client or default to reporting time
	)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	emergency.Location = location

	// Insert into database
	err = database.DB.QueryRow(
		`INSERT INTO emergencies (user_id, emergency_id, severity, latitude, longitude, issue, 
		                          media_status, location, reporting_time, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		emergency.UserID, emergency.EmergencyID, emergency.Severity, emergency.Lat, emergency.Lon,
		emergency.Issue, emergency.MediaStatus, emergency.Location, emergency.IncidentReportingTime, emergency.Status,
	).Scan(&emergency.ID)

	if err != nil {
		// Check if it's a duplicate key error
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			respondWithError(w, http.StatusConflict, "Emergency already exists")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Error creating emergency: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, emergency)
}

// reverseGeocode - Get location name from coordinates using LocationIQ
func reverseGeocode(lat, lon float64) (string, error) {
	apiKey := os.Getenv("LOCATIONIQ_API_KEY")
	if apiKey == "" {
		return fmt.Sprintf("%.6f, %.6f", lat, lon), nil // Return coordinates if no API key
	}

	// Build URL with recommended parameters
	url := fmt.Sprintf("https://us1.locationiq.com/v1/reverse?key=%s&lat=%.6f&lon=%.6f&format=json&normalizeaddress=1&addressdetails=1",
		apiKey, lat, lon)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Sprintf("%.6f, %.6f", lat, lon), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Sprintf("%.6f, %.6f", lat, lon), fmt.Errorf("locationiq API error: %d - %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("%.6f, %.6f", lat, lon), err
	}

	var locationResp LocationIQResponse
	if err := json.Unmarshal(body, &locationResp); err != nil {
		return fmt.Sprintf("%.6f, %.6f", lat, lon), err
	}

	// Build a more readable address
	address := ""
	if locationResp.Address.Road != "" {
		address += locationResp.Address.Road + ", "
	}
	if locationResp.Address.Village != "" {
		address += locationResp.Address.Village + ", "
	}
	if locationResp.Address.County != "" {
		address += locationResp.Address.County + ", "
	}
	if locationResp.Address.State != "" {
		address += locationResp.Address.State + " "
	}
	if locationResp.Address.Postcode != "" {
		address += locationResp.Address.Postcode + ", "
	}
	if locationResp.Address.Country != "" {
		address += locationResp.Address.Country
	}

	if address != "" {
		return address, nil
	}

	return locationResp.DisplayName, nil
}

// UpdateEmergencyMedia - Update emergency with media URL after upload
func UpdateEmergencyMedia(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	emergencyID := vars["id"]

	var updateData struct {
		MediaURL    string              `json:"media_url"`
		MediaStatus models.MediaStatus  `json:"media_status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	result, err := database.DB.Exec(
		`UPDATE emergencies SET media_url = $1, media_status = $2
		 WHERE id = $3`,
		updateData.MediaURL, updateData.MediaStatus, emergencyID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating emergency")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Emergency not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Media updated successfully"})
}

// GetEmergencies - Get all emergencies (supervisor view)
func GetEmergencies(w http.ResponseWriter, r *http.Request) {
	// Get query parameters for filtering
	status := r.URL.Query().Get("status")
	userID := r.URL.Query().Get("user_id")

	query := `
		SELECT e.id, e.user_id, e.emergency_id, e.severity, e.latitude, e.longitude, e.issue,
		       e.media_status, e.media_url, e.location, e.incident_time, e.reporting_time, 
		       e.status, e.resolution_time, u.name as user_name
		FROM emergencies e
		JOIN users u ON e.user_id = u.user_id
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 1

	if status != "" {
		query += fmt.Sprintf(" AND e.status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if userID != "" {
		query += fmt.Sprintf(" AND e.user_id = $%d", argCount)
		args = append(args, userID)
		argCount++
	}

	query += " ORDER BY e.reporting_time DESC LIMIT 100"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}
	defer rows.Close()

	emergencies := []map[string]interface{}{}
	for rows.Next() {
		var emergency models.Emergency
		var userName string
		err := rows.Scan(&emergency.ID, &emergency.UserID, &emergency.EmergencyID,
			&emergency.Severity, &emergency.Lat, &emergency.Lon, &emergency.Issue,
			&emergency.MediaStatus, &emergency.MediaURL, &emergency.Location,
			&emergency.IncidentTime, &emergency.IncidentReportingTime,
			&emergency.Status, &emergency.ResolutionTime, &userName)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning emergency data")
			return
		}

		emergencyMap := map[string]interface{}{
			"id":            emergency.ID,
			"user_id":       emergency.UserID,
			"user_name":     userName,
			"emergency_id":  emergency.EmergencyID,
			"severity":      emergency.Severity,
			"latitude":      emergency.Lat,
			"longitude":     emergency.Lon,
			"issue":         emergency.Issue,
			"media_status":  emergency.MediaStatus,
			"media_url":     emergency.MediaURL,
			"location":      emergency.Location,
			"incident_time": emergency.IncidentTime,
			"reporting_time": emergency.IncidentReportingTime,
			"status":        emergency.Status,
			"resolution_time": emergency.ResolutionTime,
		}
		emergencies = append(emergencies, emergencyMap)
	}

	respondWithJSON(w, http.StatusOK, emergencies)
}

// GetEmergency - Get a single emergency
func GetEmergency(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	emergencyID := vars["id"]

	var emergency models.Emergency
	var userName string
	err := database.DB.QueryRow(
		`SELECT e.id, e.user_id, e.emergency_id, e.severity, e.latitude, e.longitude, e.issue,
		        e.media_status, e.media_url, e.location, e.incident_time, e.reporting_time, 
		        e.status, e.resolution_time, u.name as user_name
		 FROM emergencies e
		 JOIN users u ON e.user_id = u.user_id
		 WHERE e.id = $1`,
		emergencyID,
	).Scan(&emergency.ID, &emergency.UserID, &emergency.EmergencyID,
		&emergency.Severity, &emergency.Lat, &emergency.Lon, &emergency.Issue,
		&emergency.MediaStatus, &emergency.MediaURL, &emergency.Location,
		&emergency.IncidentTime, &emergency.IncidentReportingTime,
		&emergency.Status, &emergency.ResolutionTime, &userName)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Emergency not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	emergencyMap := map[string]interface{}{
		"id":            emergency.ID,
		"user_id":       emergency.UserID,
		"user_name":     userName,
		"emergency_id":  emergency.EmergencyID,
		"severity":      emergency.Severity,
		"latitude":      emergency.Lat,
		"longitude":     emergency.Lon,
		"issue":         emergency.Issue,
		"media_status":  emergency.MediaStatus,
		"media_url":     emergency.MediaURL,
		"location":      emergency.Location,
		"incident_time": emergency.IncidentTime,
		"reporting_time": emergency.IncidentReportingTime,
		"status":        emergency.Status,
		"resolution_time": emergency.ResolutionTime,
	}

	respondWithJSON(w, http.StatusOK, emergencyMap)
}

// UpdateEmergencyStatus - Update emergency status
func UpdateEmergencyStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	emergencyID := vars["id"]

	var updateData struct {
		Status models.ResolutionStatus `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var resolutionTime *time.Time
	if updateData.Status == models.ResolutionComplete {
		now := time.Now()
		resolutionTime = &now
	}

	result, err := database.DB.Exec(
		`UPDATE emergencies SET status = $1, resolution_time = $2
		 WHERE id = $3`,
		updateData.Status, resolutionTime, emergencyID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating emergency status")
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Emergency not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Emergency status updated successfully",
		"status":  updateData.Status,
	})
}
