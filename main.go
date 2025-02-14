package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/shanurrahman/orchestrator/config"
	"github.com/shanurrahman/orchestrator/docker"
	_ "github.com/shanurrahman/orchestrator/docs"
	"github.com/shanurrahman/orchestrator/handlers"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	log.Println("Starting the orchestrator service...")
	
	cfg := config.Load()
	log.Println("Configuration loaded successfully")
	
	dockerClient := docker.NewDockerManager(cfg)
	log.Println("Docker manager initialized")
	
	// Create a new chi router
	r := chi.NewRouter()
	
	// Add middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	
	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
	
	// Serve Swagger documentation
	r.Handle("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
	
	// Register API routes
	r.Route("/containers", func(r chi.Router) {
		r.Post("/", handlers.CreateContainerHandler(dockerClient))
		r.Get("/{id}/status", handlers.GetContainerStatusHandler(dockerClient))
	})
	
	// Configure server with timeouts
	server := &http.Server{
		Addr:         "0.0.0.0:8090",
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	
	log.Printf("Server starting on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}