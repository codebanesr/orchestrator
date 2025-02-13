package handlers

import (
	"log"
	"net/http"

	"github.com/shanurrahman/orchestrator/docker"
)

func CreateContainerHandler(dm *docker.DockerManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request to create container from %s", r.RemoteAddr)

		containerID, err := dm.CreateContainer()
		if err != nil {
			log.Printf("Error creating container: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.Printf("Container created successfully with ID: %s", containerID)
		w.Write([]byte(containerID))
	}
}