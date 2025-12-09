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

// ==================== ADMIN AUTH ====================

type AdminSignupRequest struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Password     string `json:"password"`
	MineName     string `json:"mine_name"`
	MineLocation string `json:"mine_location"`
	AdminCode    string `json:"admin_code"`
	Role         string `json:"role"`
}

type AdminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AdminRegisterResponse struct {
	Success bool   `json:"success"`
	AdminID string `json:"admin_id"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

// RegisterAdmin - POST /api/auth/register-admin
// Create a new admin account with admin code verification
func RegisterAdmin(w http.ResponseWriter, r *http.Request) {
	var signup AdminSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&signup); err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request payload",
		})
		return
	}

	// Validate required fields
	if signup.Name == "" || signup.Email == "" || signup.Password == "" {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Name, email, and password are required",
		})
		return
	}

	// Validate admin code (hardcoded as "8888" as per app requirement)
	if signup.AdminCode != "8888" {
		respondWithJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"success": false,
			"message": "Invalid admin authorization code",
		})
		return
	}

	// Check if email already exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", signup.Email).Scan(&exists)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Database error",
		})
		return
	}
	if exists {
		respondWithJSON(w, http.StatusConflict, map[string]interface{}{
			"success": false,
			"message": "Email already registered",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Error processing password",
		})
		return
	}

	// Create admin user with mine_name as mining_site and mine_location as location
	admin, err := models.NewUser(signup.Name, signup.Email, signup.Phone, string(hashedPassword),
		signup.MineName, signup.MineLocation, models.RoleAdmin, nil)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Insert into database
	err = database.DB.QueryRow(
		`INSERT INTO users (user_id, name, email, phone, password, role, mining_site, location, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		admin.UserID, admin.Name, admin.Email, admin.Phone, admin.Password, admin.Role,
		admin.MiningSite, admin.Location, admin.CreatedAt, admin.UpdatedAt,
	).Scan(&admin.ID)

	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Error creating admin: " + err.Error(),
		})
		return
	}

	// Generate token
	token, err := middleware.GenerateToken(admin.UserID, string(admin.Role))
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Error generating token",
		})
		return
	}

	respondWithJSON(w, http.StatusCreated, AdminRegisterResponse{
		Success: true,
		AdminID: admin.UserID,
		Message: "Admin registered successfully",
		Token:   token,
	})
}

// AdminSignup - POST /api/admin/signup (legacy endpoint)
// Create a new admin account
func AdminSignup(w http.ResponseWriter, r *http.Request) {
	var signup AdminSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&signup); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate required fields
	if signup.Name == "" || signup.Email == "" || signup.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Name, email, and password are required")
		return
	}

	// Check if email already exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", signup.Email).Scan(&exists)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	if exists {
		respondWithError(w, http.StatusConflict, "Email already registered")
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(signup.Password), bcrypt.DefaultCost)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing password")
		return
	}

	// Create admin user
	admin, err := models.NewUser(signup.Name, signup.Email, signup.Phone, string(hashedPassword),
		signup.MineName, signup.MineLocation, models.RoleAdmin, nil)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Insert into database
	err = database.DB.QueryRow(
		`INSERT INTO users (user_id, name, email, phone, password, role, mining_site, location, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		admin.UserID, admin.Name, admin.Email, admin.Phone, admin.Password, admin.Role,
		admin.MiningSite, admin.Location, admin.CreatedAt, admin.UpdatedAt,
	).Scan(&admin.ID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating admin: "+err.Error())
		return
	}

	// Generate token
	token, err := middleware.GenerateToken(admin.UserID, string(admin.Role))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating token")
		return
	}

	admin.Password = "" // Don't send password back
	respondWithJSON(w, http.StatusCreated, AuthResponse{
		Token:  token,
		UserID: admin.UserID,
		Role:   string(admin.Role),
		User:   admin,
	})
}

// AdminLogin - POST /api/admin/login
// Admin login with email and password
func AdminLogin(w http.ResponseWriter, r *http.Request) {
	var login AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&login); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if login.Email == "" || login.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	// Find admin by email
	var admin models.User
	err := database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, password, role, created_at, updated_at
		 FROM users WHERE email = $1 AND role = 'ADMIN'`,
		login.Email,
	).Scan(&admin.ID, &admin.UserID, &admin.Name, &admin.Email, &admin.Phone, &admin.Password,
		&admin.Role, &admin.CreatedAt, &admin.UpdatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(login.Password)); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Generate token
	token, err := middleware.GenerateToken(admin.UserID, string(admin.Role))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating token")
		return
	}

	admin.Password = "" // Don't send password back
	respondWithJSON(w, http.StatusOK, AuthResponse{
		Token:  token,
		UserID: admin.UserID,
		Role:   string(admin.Role),
		User:   &admin,
	})
}

// ==================== ADMIN - SUPERVISOR MANAGEMENT ====================

type SupervisorCreateRequest struct {
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Phone                 string `json:"phone"`
	Password              string `json:"password"`
	SupervisorID          string `json:"supervisor_id"`
	Department            string `json:"department"`
	Role                  string `json:"role"`
	EmergencyContactName  string `json:"emergency_contact_name"`
	EmergencyContactPhone string `json:"emergency_contact_phone"`
	MiningSite            string `json:"mining_site"`
	Location              string `json:"location"`
}

type SupervisorCreateResponse struct {
	Success      bool   `json:"success"`
	SupervisorID string `json:"supervisor_id"`
	Message      string `json:"message"`
}

// AdminCreateSupervisor - POST /api/admin/supervisors
// Admin creates a new supervisor
func AdminCreateSupervisor(w http.ResponseWriter, r *http.Request) {
	var req SupervisorCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request payload",
		})
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Name, email, and password are required",
		})
		return
	}

	// Check if email already exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Database error",
		})
		return
	}
	if exists {
		respondWithJSON(w, http.StatusConflict, map[string]interface{}{
			"success": false,
			"message": "Email already registered",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Error processing password",
		})
		return
	}

	// Use department as mining_site if mining_site not provided
	miningSite := req.MiningSite
	if miningSite == "" && req.Department != "" {
		miningSite = req.Department
	}

	// Create supervisor
	supervisor, err := models.NewUser(req.Name, req.Email, req.Phone, string(hashedPassword),
		miningSite, req.Location, models.RoleSupervisor, nil)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Insert into database
	err = database.DB.QueryRow(
		`INSERT INTO users (user_id, name, email, phone, password, role, mining_site, location, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		supervisor.UserID, supervisor.Name, supervisor.Email, supervisor.Phone, supervisor.Password,
		supervisor.Role, supervisor.MiningSite, supervisor.Location, supervisor.CreatedAt, supervisor.UpdatedAt,
	).Scan(&supervisor.ID)

	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Error creating supervisor: " + err.Error(),
		})
		return
	}

	respondWithJSON(w, http.StatusCreated, SupervisorCreateResponse{
		Success:      true,
		SupervisorID: supervisor.UserID,
		Message:      "Supervisor added successfully",
	})
}

// AdminGetSupervisors - GET /api/admin/supervisors
// Admin gets all supervisors
func AdminGetSupervisors(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, created_at, updated_at
		 FROM users WHERE role = 'SUPERVISOR' ORDER BY created_at DESC`,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	type SupervisorResponse struct {
		SupervisorID string `json:"supervisor_id"`
		Name         string `json:"name"`
		Email        string `json:"email"`
		Phone        string `json:"phone"`
		Department   string `json:"department"`
		Role         string `json:"role"`
		Status       string `json:"status"`
	}

	supervisors := []SupervisorResponse{}
	for rows.Next() {
		var sup models.User
		err := rows.Scan(&sup.ID, &sup.UserID, &sup.Name, &sup.Email, &sup.Phone,
			&sup.Role, &sup.MiningSite, &sup.Location, &sup.CreatedAt, &sup.UpdatedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning supervisor")
			return
		}
		supervisors = append(supervisors, SupervisorResponse{
			SupervisorID: sup.UserID,
			Name:         sup.Name,
			Email:        sup.Email,
			Phone:        sup.Phone,
			Department:   sup.MiningSite, // Using mining_site as department
			Role:         string(sup.Role),
			Status:       "active",
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"supervisors": supervisors,
	})
}

// AdminGetSupervisor - GET /api/admin/supervisors/{id}
// Admin gets a single supervisor by ID
func AdminGetSupervisor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	supervisorID := vars["id"]

	var supervisor models.User
	err := database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, created_at, updated_at
		 FROM users WHERE user_id = $1 AND role = 'SUPERVISOR'`,
		supervisorID,
	).Scan(&supervisor.ID, &supervisor.UserID, &supervisor.Name, &supervisor.Email, &supervisor.Phone,
		&supervisor.Role, &supervisor.MiningSite, &supervisor.Location, &supervisor.CreatedAt, &supervisor.UpdatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Supervisor not found")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, supervisor)
}

// AdminUpdateSupervisor - PUT /api/admin/supervisors/{id}
// Admin updates a supervisor
func AdminUpdateSupervisor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	supervisorID := vars["id"]

	var updateData struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Phone      string `json:"phone"`
		MiningSite string `json:"mining_site"`
		Location   string `json:"location"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Check if supervisor exists
	var exists bool
	err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND role = 'SUPERVISOR')",
		supervisorID,
	).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Supervisor not found")
		return
	}

	result, err := database.DB.Exec(
		`UPDATE users SET name = $1, email = $2, phone = $3, mining_site = $4, location = $5, updated_at = NOW()
		 WHERE user_id = $6 AND role = 'SUPERVISOR'`,
		updateData.Name, updateData.Email, updateData.Phone, updateData.MiningSite, updateData.Location, supervisorID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating supervisor: "+err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Supervisor not found")
		return
	}

	// Fetch updated supervisor
	var supervisor models.User
	err = database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, created_at, updated_at
		 FROM users WHERE user_id = $1`,
		supervisorID,
	).Scan(&supervisor.ID, &supervisor.UserID, &supervisor.Name, &supervisor.Email, &supervisor.Phone,
		&supervisor.Role, &supervisor.MiningSite, &supervisor.Location, &supervisor.CreatedAt, &supervisor.UpdatedAt)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching updated supervisor")
		return
	}

	respondWithJSON(w, http.StatusOK, supervisor)
}

// AdminDeleteSupervisor - DELETE /api/admin/supervisors/{id}
// Admin deletes a supervisor
func AdminDeleteSupervisor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	supervisorID := vars["id"]

	result, err := database.DB.Exec(
		"DELETE FROM users WHERE user_id = $1 AND role = 'SUPERVISOR'",
		supervisorID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error: "+err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Supervisor not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Supervisor deleted successfully",
	})
}

// ==================== ADMIN - MINER MANAGEMENT ====================

type MinerCreateByAdminRequest struct {
	Name                  string `json:"name"`
	Email                 string `json:"email"`
	Phone                 string `json:"phone"`
	Password              string `json:"password"`
	MinerID               string `json:"miner_id"`
	DateOfBirth           string `json:"date_of_birth"`
	Zone                  string `json:"zone"`
	Role                  string `json:"role"`
	EmergencyContactName  string `json:"emergency_contact_name"`
	EmergencyContactPhone string `json:"emergency_contact_phone"`
	SupervisorID          string `json:"supervisor_id"`
	MiningSite            string `json:"mining_site"`
	Location              string `json:"location"`
}

type MinerCreateResponse struct {
	Success bool   `json:"success"`
	MinerID string `json:"miner_id"`
	Message string `json:"message"`
}

// AdminCreateMiner - POST /api/admin/miners
// Admin creates a new miner
func AdminCreateMiner(w http.ResponseWriter, r *http.Request) {
	var req MinerCreateByAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Invalid request payload",
		})
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": "Name, email, and password are required",
		})
		return
	}

	// Check if email already exists
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Database error",
		})
		return
	}
	if exists {
		respondWithJSON(w, http.StatusConflict, map[string]interface{}{
			"success": false,
			"message": "Email already registered",
		})
		return
	}

	// If supervisor_id is provided, verify it exists
	var supervisorID *string
	if req.SupervisorID != "" {
		var supExists bool
		err := database.DB.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND role = 'SUPERVISOR')",
			req.SupervisorID,
		).Scan(&supExists)
		if err != nil {
			respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"success": false,
				"message": "Database error",
			})
			return
		}
		if !supExists {
			respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"message": "Supervisor not found",
			})
			return
		}
		supervisorID = &req.SupervisorID

		// Get mining_site and location from supervisor if not provided
		if req.MiningSite == "" || req.Location == "" {
			database.DB.QueryRow(
				"SELECT mining_site, location FROM users WHERE user_id = $1",
				req.SupervisorID,
			).Scan(&req.MiningSite, &req.Location)
		}
	}

	// Use zone as mining_site if mining_site not provided
	miningSite := req.MiningSite
	if miningSite == "" && req.Zone != "" {
		miningSite = req.Zone
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Error processing password",
		})
		return
	}

	// Create miner
	miner, err := models.NewUser(req.Name, req.Email, req.Phone, string(hashedPassword),
		miningSite, req.Location, models.RoleMiner, supervisorID)
	if err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Insert into database
	err = database.DB.QueryRow(
		`INSERT INTO users (user_id, name, email, phone, password, role, mining_site, location, supervisor_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		miner.UserID, miner.Name, miner.Email, miner.Phone, miner.Password,
		miner.Role, miner.MiningSite, miner.Location, miner.SupervisorID, miner.CreatedAt, miner.UpdatedAt,
	).Scan(&miner.ID)

	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Error creating miner: " + err.Error(),
		})
		return
	}

	respondWithJSON(w, http.StatusCreated, MinerCreateResponse{
		Success: true,
		MinerID: miner.UserID,
		Message: "Miner added successfully",
	})
}

// AdminGetMiners - GET /api/admin/miners
// Admin gets all miners (optionally filtered by supervisor_id query param)
func AdminGetMiners(w http.ResponseWriter, r *http.Request) {
	supervisorID := r.URL.Query().Get("supervisor_id")

	var rows *sql.Rows
	var err error

	if supervisorID != "" {
		rows, err = database.DB.Query(
			`SELECT id, user_id, name, email, phone, role, mining_site, location, supervisor_id, created_at, updated_at
			 FROM users WHERE role = 'MINER' AND supervisor_id = $1 ORDER BY created_at DESC`,
			supervisorID,
		)
	} else {
		rows, err = database.DB.Query(
			`SELECT id, user_id, name, email, phone, role, mining_site, location, supervisor_id, created_at, updated_at
			 FROM users WHERE role = 'MINER' ORDER BY created_at DESC`,
		)
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	type MinerResponse struct {
		MinerID string `json:"miner_id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Zone    string `json:"zone"`
		Role    string `json:"role"`
		Status  string `json:"status"`
	}

	miners := []MinerResponse{}
	for rows.Next() {
		var miner models.User
		err := rows.Scan(&miner.ID, &miner.UserID, &miner.Name, &miner.Email, &miner.Phone,
			&miner.Role, &miner.MiningSite, &miner.Location, &miner.SupervisorID, &miner.CreatedAt, &miner.UpdatedAt)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Error scanning miner")
			return
		}
		miners = append(miners, MinerResponse{
			MinerID: miner.UserID,
			Name:    miner.Name,
			Email:   miner.Email,
			Phone:   miner.Phone,
			Zone:    miner.MiningSite, // Using mining_site as zone
			Role:    string(miner.Role),
			Status:  "active",
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"miners": miners,
	})
}

// AdminGetMiner - GET /api/admin/miners/{id}
// Admin gets a single miner by ID
func AdminGetMiner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	minerID := vars["id"]

	var miner models.User
	err := database.DB.QueryRow(
		`SELECT id, user_id, name, email, phone, role, mining_site, location, supervisor_id, created_at, updated_at
		 FROM users WHERE user_id = $1 AND role = 'MINER'`,
		minerID,
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

// AdminUpdateMiner - PUT /api/admin/miners/{id}
// Admin updates a miner
func AdminUpdateMiner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	minerID := vars["id"]

	var updateData struct {
		Name         string `json:"name"`
		Email        string `json:"email"`
		Phone        string `json:"phone"`
		MiningSite   string `json:"mining_site"`
		Location     string `json:"location"`
		SupervisorID string `json:"supervisor_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Check if miner exists
	var exists bool
	err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND role = 'MINER')",
		minerID,
	).Scan(&exists)
	if err != nil || !exists {
		respondWithError(w, http.StatusNotFound, "Miner not found")
		return
	}

	// If supervisor_id is provided, verify it exists
	if updateData.SupervisorID != "" {
		var supExists bool
		err := database.DB.QueryRow(
			"SELECT EXISTS(SELECT 1 FROM users WHERE user_id = $1 AND role = 'SUPERVISOR')",
			updateData.SupervisorID,
		).Scan(&supExists)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Database error")
			return
		}
		if !supExists {
			respondWithError(w, http.StatusBadRequest, "Supervisor not found")
			return
		}
	}

	result, err := database.DB.Exec(
		`UPDATE users SET name = $1, email = $2, phone = $3, mining_site = $4, location = $5, supervisor_id = $6, updated_at = NOW()
		 WHERE user_id = $7 AND role = 'MINER'`,
		updateData.Name, updateData.Email, updateData.Phone, updateData.MiningSite, updateData.Location, updateData.SupervisorID, minerID,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error updating miner: "+err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "Miner not found")
		return
	}

	// Fetch updated miner
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

// AdminDeleteMiner - DELETE /api/admin/miners/{id}
// Admin deletes a miner
func AdminDeleteMiner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	minerID := vars["id"]

	result, err := database.DB.Exec(
		"DELETE FROM users WHERE user_id = $1 AND role = 'MINER'",
		minerID,
	)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "Database error: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"message": "Miner not found",
		})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Miner deleted successfully",
	})
}
