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

func (dm *DockerManager) registerWithConsul(containerID string, containerIP string) error {
	// Register UI endpoint
	uiRegistration := ConsulServiceRegistration{
		Name:    fmt.Sprintf("chrome-ui-%s", containerID[:12]),
		ID:      fmt.Sprintf("chrome-ui-%s", containerID[:12]),
		Address: containerIP,
		Port:    4000,
		Tags:    []string{fmt.Sprintf("urlprefix-/%s/", containerID[:12])},
	}
	uiRegistration.Check.HTTP = fmt.Sprintf("http://%s:4000/health", containerIP)
	uiRegistration.Check.Interval = "10s"

	// Register Debug endpoint
	debugRegistration := ConsulServiceRegistration{
		Name:    fmt.Sprintf("chrome-debug-%s", containerID[:12]),
		ID:      fmt.Sprintf("chrome-debug-%s", containerID[:12]),
		Address: containerIP,
		Port:    9222,
		Tags:    []string{fmt.Sprintf("urlprefix-/%s/debug/", containerID[:12])},
	}
	debugRegistration.Check.HTTP = fmt.Sprintf("http://%s:4000/health", containerIP)
	debugRegistration.Check.Interval = "10s"

	// Register both services
	for _, registration := range []ConsulServiceRegistration{uiRegistration, debugRegistration} {
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
	}

	log.Printf("Successfully registered container %s services with Consul", containerID[:12])
	return nil
}

// Add this new type at the top with other types
type ContainerEndpoints struct {
    ContainerID  string `json:"container_id"`
    UIPath       string `json:"ui_path"`
    DebugPath    string `json:"debug_path"`
}

// Modify the CreateContainer function signature and return
func (dm *DockerManager) CreateContainer() (*ContainerEndpoints, error) {
    containerID := utils.GenerateID()
    shortID := containerID[:12]
    log.Printf("Creating new container with ID: %s", shortID)

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"4000/tcp": []nat.PortBinding{{HostPort: ""}}, // UI port
			"9222/tcp": []nat.PortBinding{{HostPort: ""}}, // Debug port
		},
		NetworkMode: container.NetworkMode(dm.network),
	}

    resp, err := dm.cli.ContainerCreate(
        context.Background(),
        &container.Config{
            Image:        "custom-chrome-ui:latest",
            ExposedPorts: nat.PortSet{"4000/tcp": struct{}{}, "9222/tcp": struct{}{}},
        },
        hostConfig,
        nil,
        nil,
        "",
    )

    if err != nil {
        return nil, fmt.Errorf("failed to create container: %v", err)
    }

    // Start the container
    if err := dm.cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{}); err != nil {
        return nil, fmt.Errorf("failed to start container: %v", err)
    }

    // Get container IP address
    inspect, err := dm.cli.ContainerInspect(context.Background(), resp.ID)
    if err != nil {
        return nil, fmt.Errorf("failed to inspect container: %v", err)
    }

	containerIP := inspect.NetworkSettings.Networks[dm.network].IPAddress

	// Register services with Consul
	if err := dm.registerWithConsul(resp.ID, containerIP); err != nil {
		log.Printf("Warning: Failed to register container with Consul: %v", err)
		// Optionally clean up the container if registration fails
		// dm.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
		// return "", err
	}

    // After successful container creation and registration
    endpoints := &ContainerEndpoints{
        ContainerID: shortID,
        UIPath:      fmt.Sprintf("/%s/", shortID),
        DebugPath:   fmt.Sprintf("/%s/debug/", shortID),
    }

    return endpoints, nil
}