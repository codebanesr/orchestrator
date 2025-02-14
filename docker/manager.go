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
    cli     *client.Client
    cfg     *config.Config
    network string // Fabio network name
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
        network: "fabio_network",
    }
}

func (dm *DockerManager) CreateContainer() (string, error) {
    containerID := utils.GenerateID()
    log.Printf("Creating new container with ID: %s", containerID)
    
    // Labels for both Fabio routing and Consul service registration
    labels := map[string]string{
        // Fabio routing labels
        "urlprefix-/" + containerID + "/ui": "proto=http dst=:4000",
        "urlprefix-/" + containerID + "/debug": "proto=http dst=:9222",
        
        // Consul service registration labels
        "consul.service.name": "chrome-instance-" + containerID,
        "consul.service.tags": "urlprefix-/" + containerID + "/ui proto=http dst=:4000," +
                              "urlprefix-/" + containerID + "/debug proto=http dst=:9222",
    }

    hostConfig := &container.HostConfig{
        PortBindings: nat.PortMap{
            "4000/tcp": []nat.PortBinding{{HostPort: ""}},
            "9222/tcp": []nat.PortBinding{{HostPort: ""}},
        },
        NetworkMode: container.NetworkMode(dm.network), // Connect to Fabio network
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