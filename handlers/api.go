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

// Add new types
type CreateContainerRequest struct {
    ImageID string `json:"image_id" validate:"required"`
}

// Add new handler
func ListImagesHandler(dm *docker.DockerManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        images := dm.ListAvailableImages()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(images)
    }
}

// Modify CreateContainerHandler
func CreateContainerHandler(dm *docker.DockerManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req CreateContainerRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request body", http.StatusBadRequest)
            return
        }

        containerID, err := dm.CreateContainerAsync(req.ImageID)
        if err != nil {
            log.Printf("Error initiating container creation: %v", err)
            http.Error(w, err.Error(), http.StatusBadRequest)
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