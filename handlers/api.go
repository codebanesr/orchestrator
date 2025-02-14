package handlers

import (
	"encoding/json"
	"log"
	"net/http"

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
func CreateContainerHandler(dm *docker.DockerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request to create container from %s", r.RemoteAddr)

		containerEndpoints, err := dm.CreateContainer()
		if err != nil {
			log.Printf("Error creating container: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Convert container endpoints to JSON
		jsonResponse, err := json.Marshal(containerEndpoints)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Printf("Container created successfully with endpoints: %+v", containerEndpoints)
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse)
	}
}