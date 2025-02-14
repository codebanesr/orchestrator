package docker

import (
	"context"
	"io"
	"log"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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

type ConsulServiceRegistration struct {
    Name    string   `json:"Name"`
    ID      string   `json:"ID"`
    Address string   `json:"Address"`
    Port    int      `json:"Port"`
    Tags    []string `json:"Tags"`
    Check   struct {
        HTTP     string `json:"HTTP"`
        Interval string `json:"Interval"`
    } `json:"Check"`
}

func (dm *DockerManager) registerContainerWithConsul(containerID string) error {
    registration := ConsulServiceRegistration{
        Name:    fmt.Sprintf("chrome-instance-%s", containerID),
        ID:      fmt.Sprintf("chrome-%s", containerID),
        Address: containerID, // Using container ID as address
        Port:    4000,       // Main UI port
        Tags: []string{
            fmt.Sprintf("urlprefix-/%s/ui proto=http dst=:4000", containerID),
            fmt.Sprintf("urlprefix-/%s/debug proto=http dst=:9222", containerID),
        },
    }
    registration.Check.HTTP = fmt.Sprintf("http://%s:4000/health", containerID)
    registration.Check.Interval = "10s"

    jsonData, err := json.Marshal(registration)
    if err != nil {
        return fmt.Errorf("failed to marshal registration data: %v", err)
    }

    resp, err := http.DefaultClient.Do(&http.Request{
        Method: "PUT",
        URL:    &url.URL{Scheme: "http", Host: "localhost:8500", Path: "/v1/agent/service/register"},
        Body:   io.NopCloser(bytes.NewReader(jsonData)),
    })
    if err != nil {
        return fmt.Errorf("failed to register service: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("failed to register service, status: %d", resp.StatusCode)
    }

    log.Printf("Successfully registered container %s with Consul", containerID)
    return nil
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

    // Start the container
    if err := dm.cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
        log.Printf("Failed to start container: %v", err)
        return "", err
    }

    // Register the container with Consul
    if err := dm.registerContainerWithConsul(resp.ID); err != nil {
        log.Printf("Warning: Failed to register container with Consul: %v", err)
        // Optionally, you might want to clean up the container if registration fails
        // dm.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
        // return "", err
    }

    log.Printf("Container created and registered successfully with ID: %s", resp.ID)
    return resp.ID, nil
}