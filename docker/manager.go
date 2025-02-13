package docker

import (
	"context"
	"log"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/shanurrahman/orchestrator/config"
	"github.com/shanurrahman/orchestrator/utils"
)

type DockerManager struct {
  cli      *client.Client
  cfg      *config.Config
  network  string // Traefik network name
}

func NewDockerManager(cfg *config.Config) *DockerManager {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Printf("Error creating Docker client: %v", err)
		return nil
	}
	log.Println("Docker client initialized successfully")
	return &DockerManager{
		cli:     cli,
		cfg:     cfg,
		network: "traefik_network",
	}
}

func (dm *DockerManager) CreateContainer() (string, error) {
	containerID := utils.GenerateID()
	log.Printf("Creating new container with ID: %s", containerID)
	
	labels := map[string]string{
		// UI Endpoint
		"traefik.http.routers."+containerID+"-ui.rule": `PathPrefix("/`+containerID+`/ui")`,
		"traefik.http.services."+containerID+"-ui.loadbalancer.server.port": "4000",
		"traefik.http.middlewares."+containerID+"-ui-strip.stripprefix.prefixes": "/"+containerID+"/ui",

		// Debug Endpoint  
		"traefik.http.routers."+containerID+"-debug.rule": `PathPrefix("/`+containerID+`/debug")`,
		"traefik.http.services."+containerID+"-debug.loadbalancer.server.port": "9222",
		"traefik.http.middlewares."+containerID+"-debug-strip.stripprefix.prefixes": "/"+containerID+"/debug",

		// Common Security
		"traefik.http.routers."+containerID+"-ui.middlewares": "auth@file",
		"traefik.http.routers."+containerID+"-debug.middlewares": "auth@file",
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"4000/tcp": []nat.PortBinding{{HostPort: ""}},
			"9222/tcp": []nat.PortBinding{{HostPort: ""}},
		},
	}

	resp, err := dm.cli.ContainerCreate(context.Background(), &container.Config{
		Image:        "custom-chrome-ui:latest",
		ExposedPorts: map[nat.Port]struct{}{"4000/tcp": {}, "9222/tcp": {}},
		Labels:       labels,
	}, hostConfig, nil, nil, "")

	if err != nil {
		log.Printf("Failed to create container: %v", err)
		return "", err
	}
	log.Printf("Container created successfully with ID: %s", resp.ID)

	return resp.ID, nil
}