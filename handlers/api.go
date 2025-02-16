package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shanurrahman/orchestrator/config"
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
// CreateContainerRequest represents the request body for container creation
// @Description Request body for creating a new container
type CreateContainerRequest struct {
    // The ID of the image to use for the container
    // @example ubuntu-base
    ImageID     string            `json:"image_id" validate:"required"`
    VNCConfig   config.VNCConfig  `json:"vnc_config,omitempty"`
}

// Add new handler
// ListImagesHandler godoc
// @Summary     List available images
// @Description Get a list of all available container images
// @Tags        images
// @Accept      json
// @Produce     json
// @Success     200 {array}  docker.ImageInfo
// @Router      /containers/images [get]
func ListImagesHandler(dm *docker.DockerManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        images := dm.ListAvailableImages()
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(images)
    }
}

// Modify CreateContainerHandler
// CreateContainerHandler godoc
// @Summary     Create a new container
// @Description Create a new container from a specified image
// @Tags        containers
// @Accept      json
// @Produce     json
// @Param       request body CreateContainerRequest true "Container creation request"
// @Success     200 {object} CreateContainerResponse
// @Failure     400 {string} string "Bad Request"
// @Failure     500 {string} string "Internal Server Error"
// @Router      /containers [post]
func CreateContainerHandler(dm *docker.DockerManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req CreateContainerRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request body", http.StatusBadRequest)
            return
        }

        config := docker.ContainerConfig{
            ImageID:   req.ImageID,
            VNCConfig: req.VNCConfig,
        }

        containerID, err := dm.CreateContainerAsync(config)
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

// GetContainerStatusHandler godoc
// @Summary     Get container status
// @Description Get the current status of a container
// @Tags        containers
// @Accept      json
// @Produce     json
// @Param       id path string true "Container ID"
// @Success     200 {object} docker.ContainerStatus
// @Failure     404 {string} string "Container not found"
// @Router      /containers/{id}/status [get]
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

func KillContainerHandler(dm *docker.DockerManager) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        containerID := chi.URLParam(r, "id")
        if containerID == "" {
            http.Error(w, "container ID is required", http.StatusBadRequest)
            return
        }

        err := dm.KillContainer(containerID)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"message": "Container killed successfully"})
    }
}