package main

import (
	"log"
	"net/http"
	"time"

	"github.com/shanurrahman/orchestrator/config"
	"github.com/shanurrahman/orchestrator/docker"
	"github.com/shanurrahman/orchestrator/handlers"
)

// Add logging middleware to track all incoming requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("Received %s request for %s", r.Method, r.URL.Path)
		
		// Create a custom response writer to capture status code
		rw := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		
		duration := time.Since(start)
		log.Printf("Completed %s %s | Status: %d | Duration: %v",
			r.Method, r.URL.Path, rw.status, duration)
	})
}

// Custom response writer to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	log.Println("Starting the orchestrator service...")
	
	cfg := config.Load()
	log.Println("Configuration loaded successfully")
	
	dockerClient := docker.NewDockerManager(cfg)
	log.Println("Docker manager initialized")
	
	// Create a new router and wrap it with logging
	router := http.NewServeMux()
	
	// Register health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	// Register handlers with explicit method checks
	router.HandleFunc("/containers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handlers.CreateContainerHandler(dockerClient)(w, r)
	})
	
	// Add catch-all route for unhandled paths
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Unhandled route accessed: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})
	
	// Configure server with timeouts and logging
	server := &http.Server{
		Addr:         "0.0.0.0:8090",
		Handler:      loggingMiddleware(router),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	
	log.Printf("Server starting on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}