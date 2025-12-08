package main

import (
	"MineSafeBackend/database"
	"MineSafeBackend/handlers"
	"MineSafeBackend/middleware"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	// Initialize database
	if err := database.InitDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer database.CloseDB()

	// Initialize JWT
	middleware.InitJWT()

	// Initialize rate limiter (100 requests per minute)
	middleware.InitRateLimiter(100)

	// Create router
	router := mux.NewRouter()

	// Serve uploaded files (videos, profile pictures)
	router.PathPrefix("/uploads/").Handler(http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Public routes
	router.HandleFunc("/api/health", healthCheck).Methods("GET")
	router.HandleFunc("/api/auth/signup", handlers.SupervisorSignup).Methods("POST")
	router.HandleFunc("/api/auth/login", handlers.Login).Methods("POST")
	router.HandleFunc("/api/app/miner/login", handlers.MinerAppLogin).Methods("POST")

	// Protected routes
	api := router.PathPrefix("/api").Subrouter()
	api.Use(middleware.AuthMiddleware)

	// ==================== VIDEO FEED & RECOMMENDATIONS ====================
	// GET /api/videos/feed?page=1&limit=10 - Paginated video feed (TikTok-style)
	api.HandleFunc("/videos/feed", handlers.GetVideoFeed).Methods("GET")
	// GET /api/videos/recommended?tags=PPE,safety - Tag-based recommendations
	api.HandleFunc("/videos/recommended", handlers.GetRecommendedVideos).Methods("GET")
	// POST /api/videos/{id}/like - Like a video
	api.HandleFunc("/videos/{id}/like", handlers.LikeVideo).Methods("POST")
	// POST /api/videos/{id}/dislike - Dislike a video
	api.HandleFunc("/videos/{id}/dislike", handlers.DislikeVideo).Methods("POST")
	// POST /api/videos/upload - Upload video with optional quiz (multipart)
	api.HandleFunc("/videos/upload", handlers.UploadVideo).Methods("POST")

	// ==================== TRAINING & QUIZ ====================
	// GET /api/training/quiz?title=Safety%20Helmet%20Usage - Get quiz by video title
	api.HandleFunc("/training/quiz", handlers.GetQuizByTitle).Methods("GET")
	// GET /api/training/quizzes - Get list of all quizzes
	api.HandleFunc("/training/quizzes", handlers.GetQuizList).Methods("GET")

	// ==================== USER TAGS ====================
	// GET /api/user/tags - Get user's interest tags
	api.HandleFunc("/user/tags", handlers.GetUserTags).Methods("GET")
	// PUT /api/user/tags - Update user's interest tags
	api.HandleFunc("/user/tags", handlers.UpdateUserTags).Methods("PUT")

	// ==================== USER PROFILE (App) ====================
	// GET /api/app/profile - Get full user profile with tags
	api.HandleFunc("/app/profile", handlers.GetUserProfile).Methods("GET")
	// PUT /api/app/profile - Update user profile
	api.HandleFunc("/app/profile", handlers.UpdateUserProfile).Methods("PUT")
	// POST /api/app/profile/picture - Upload profile picture
	api.HandleFunc("/app/profile/picture", handlers.UploadProfilePicture).Methods("POST")

	// App routes (User protected - MINER/OPERATOR)
	api.HandleFunc("/app/quiz-calendar", handlers.GetQuizCalendarAndStreak).Methods("GET")
	api.HandleFunc("/app/checklists/pre-start", handlers.GetPreStartChecklistForApp).Methods("GET")
	api.HandleFunc("/app/checklists/pre-start/complete", handlers.UpdatePreStartChecklistForApp).Methods("PUT")
	api.HandleFunc("/app/checklists/ppe", handlers.GetPPEChecklistForApp).Methods("GET")
	api.HandleFunc("/app/checklists/ppe/complete", handlers.UpdatePPEChecklistForApp).Methods("PUT")

	// User routes
	api.HandleFunc("/me", handlers.GetMe).Methods("GET")

	// Miner management routes (supervisor only)
	minerRoutes := api.PathPrefix("/miners").Subrouter()
	minerRoutes.Use(middleware.SupervisorOnly)
	minerRoutes.HandleFunc("", handlers.CreateMiner).Methods("POST")
	minerRoutes.HandleFunc("", handlers.GetMiners).Methods("GET")
	minerRoutes.HandleFunc("/{id}", handlers.GetMiner).Methods("GET")
	minerRoutes.HandleFunc("/{id}", handlers.UpdateMiner).Methods("PUT")
	minerRoutes.HandleFunc("/{id}", handlers.DeleteMiner).Methods("DELETE")

	// Video module routes
	api.HandleFunc("/modules", handlers.GetVideoModules).Methods("GET")
	api.HandleFunc("/modules/{id}", handlers.GetVideoModule).Methods("GET")
	api.HandleFunc("/modules/{id}/questions", handlers.GetQuestions).Methods("GET")
	api.HandleFunc("/modules/submit", handlers.SubmitModuleAnswers).Methods("POST")
	api.HandleFunc("/modules/star", handlers.GetStarVideo).Methods("GET")

	// Video module management (supervisor only)
	moduleManagement := api.PathPrefix("/modules").Subrouter()
	moduleManagement.Use(middleware.SupervisorOnly)
	moduleManagement.HandleFunc("", handlers.CreateVideoModule).Methods("POST")
	moduleManagement.HandleFunc("/{id}/star", handlers.SetStarVideo).Methods("POST")
	moduleManagement.HandleFunc("/questions", handlers.CreateQuestion).Methods("POST")

	// Learning streak routes
	api.HandleFunc("/streaks", handlers.GetLearningStreaks).Methods("GET")
	api.HandleFunc("/streak/me", handlers.GetMinerStreak).Methods("GET")
	api.HandleFunc("/completions/me", handlers.GetMinerCompletions).Methods("GET")

	// Checklist management routes (supervisor only)
	checklistRoutes := api.PathPrefix("/checklists").Subrouter()
	checklistRoutes.Use(middleware.SupervisorOnly)
	// Pre-Start Checklist (Supervisor)
	checklistRoutes.HandleFunc("/pre-start", handlers.CreatePreStartChecklistItem).Methods("POST")
	checklistRoutes.HandleFunc("/pre-start", handlers.GetPreStartChecklistItems).Methods("GET")
	checklistRoutes.HandleFunc("/pre-start/{id}", handlers.DeletePreStartChecklistItem).Methods("DELETE")
	checklistRoutes.HandleFunc("/pre-start/complete", handlers.UpdatePreStartChecklistCompletion).Methods("PUT")
	// PPE Checklist (Supervisor)
	checklistRoutes.HandleFunc("/ppe", handlers.CreatePPEChecklistItem).Methods("POST")
	checklistRoutes.HandleFunc("/ppe", handlers.GetPPEChecklistItems).Methods("GET")
	checklistRoutes.HandleFunc("/ppe/{id}", handlers.DeletePPEChecklistItem).Methods("DELETE")
	checklistRoutes.HandleFunc("/ppe/complete", handlers.UpdatePPEChecklistCompletion).Methods("PUT")

	// Dashboard routes (supervisor only)
	dashboardRoutes := api.PathPrefix("/dashboard").Subrouter()
	dashboardRoutes.Use(middleware.SupervisorOnly)
	dashboardRoutes.HandleFunc("/stats", handlers.GetDashboardStats).Methods("GET")

	// Emergency routes
	api.HandleFunc("/emergencies", handlers.CreateEmergency).Methods("POST")
	api.HandleFunc("/emergencies", handlers.GetEmergencies).Methods("GET")
	api.HandleFunc("/emergencies/{id}", handlers.GetEmergency).Methods("GET")
	api.HandleFunc("/emergencies/{id}/media", handlers.UpdateEmergencyMedia).Methods("PUT")
	api.HandleFunc("/emergencies/{id}/status", handlers.UpdateEmergencyStatus).Methods("PUT")

	//app routes
	//integrations := router.PathPrefix("/application").Subrouter()
	//integrations.Use(middleware.ServiceAuthMiddleware)
	//integrations.HandleFunc("/login", handlers.ApplicationHandler).Methods("POST")

	// Apply logging middleware
	router.Use(middleware.LoggingMiddleware)
	router.Use(middleware.RateLimitMiddleware)

	// Configure CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: getAllowedOrigins(),
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
		},
		ExposedHeaders: []string{
			"Link",
		},
		AllowCredentials: true,
		MaxAge:           300,
	})

	handler := corsHandler.Handler(router)

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"MineSafe Backend"}`))
}

func getAllowedOrigins() []string {
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		// Default allowed origins for development
		return []string{
			"*",
		}
	}

	// Parse comma-separated origins from environment
	return parseCommaSeparated(origins)
}

func parseCommaSeparated(s string) []string {
	if s == "" {
		return []string{}
	}

	var result []string
	current := ""
	for _, char := range s {
		if char == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if char != ' ' {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
