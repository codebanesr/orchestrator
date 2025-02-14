package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shanurrahman/orchestrator/docker"
)

// CreateContainerHandler handles the creation of a new container
// @Summary Create a new container
// @Description Creates a new container instance and returns its access endpoints
// @Tags containers
// @Accept json
// @Produce json
// @Success 200 {object} models.ContainerResponse
// @Failure 500 {string} string "Internal Server Error"
// @Router /containers [post]
// Add these types
type CreateContainerResponse struct {
    ContainerID string `json:"container_id"`
    StatusURL   string `json:"status_url"`
}

func CreateContainerHandler(dm *docker.DockerManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        log.Printf("Received request to create container from %s", r.RemoteAddr)

        containerID, err := dm.CreateContainerAsync()
        if err != nil {
            log.Printf("Error initiating container creation: %v", err)
            http.Error(w, "Failed to initiate container creation", http.StatusInternalServerError)
            return
        }

        response := CreateContainerResponse{
            ContainerID: containerID,
            StatusURL:   fmt.Sprintf("/containers/%s/status", containerID),
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
    }
}

func GetContainerStatusHandler(dm *docker.DockerManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        containerID := chi.URLParam(r, "id")
        status := dm.GetContainerStatus(containerID)
        
        if status == nil {
            http.Error(w, "Container not found", http.StatusNotFound)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(status)
    }
}