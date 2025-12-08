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

	// Public routes
	router.HandleFunc("/api/health", healthCheck).Methods("GET")
	router.HandleFunc("/api/auth/signup", handlers.SupervisorSignup).Methods("POST")
	router.HandleFunc("/api/auth/login", handlers.Login).Methods("POST")
	router.HandleFunc("/api/app/miner/login", handlers.MinerAppLogin).Methods("POST")

	// Protected routes
	api := router.PathPrefix("/api").Subrouter()
	api.Use(middleware.AuthMiddleware)

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
