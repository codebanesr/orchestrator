package docker

import (
	"context"
	"io"
	"log"
	"sync"

	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/shanurrahman/orchestrator/config"
	"github.com/shanurrahman/orchestrator/utils"
)

// Add these types at the top with other types
type ContainerStatus struct {
    Status      string `json:"status"`
    Message     string `json:"message"`
    Endpoints   *ContainerEndpoints `json:"endpoints,omitempty"`
    Error       string `json:"error,omitempty"`
}

type containerStatusMap struct {
    sync.RWMutex
    statuses map[string]*ContainerStatus
}

// Add this to DockerManager struct
type DockerManager struct {
    cli            *client.Client
    cfg            *config.Config
    network        string
    containerStats containerStatusMap
}

// Update NewDockerManager
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
        containerStats: containerStatusMap{
            statuses: make(map[string]*ContainerStatus),
        },
    }
}

// Add these new methods
func (dm *DockerManager) GetContainerStatus(id string) *ContainerStatus {
    dm.containerStats.RLock()
    defer dm.containerStats.RUnlock()
    return dm.containerStats.statuses[id]
}

func (dm *DockerManager) CreateContainerAsync() (string, error) {
    containerID := utils.GenerateID()
    shortID := containerID[:12]

    // Initialize status
    dm.containerStats.Lock()
    dm.containerStats.statuses[shortID] = &ContainerStatus{
        Status:  "initializing",
        Message: "Starting container creation",
    }
    dm.containerStats.Unlock()

    // Start async container creation
    go dm.handleContainerCreation(shortID)

    return shortID, nil
}

func (dm *DockerManager) handleContainerCreation(shortID string) {
    updateStatus := func(status, message string, err error) {
        dm.containerStats.Lock()
        defer dm.containerStats.Unlock()
        
        if s, exists := dm.containerStats.statuses[shortID]; exists {
            s.Status = status
            s.Message = message
            if err != nil {
                s.Error = err.Error()
            }
        }
    }

    // Create container using existing logic
    endpoints, err := dm.CreateContainer()
    if err != nil {
        updateStatus("failed", "Container creation failed", err)
        return
    }

    updateStatus("ready", "Container is ready", nil)
    dm.containerStats.Lock()
    dm.containerStats.statuses[shortID].Endpoints = endpoints
    dm.containerStats.Unlock()
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
    // Register UI endpoint (VNC viewer)
    uiRegistration := ConsulServiceRegistration{
        Name:    fmt.Sprintf("chrome-ui-%s", containerID[:12]),
        ID:      fmt.Sprintf("chrome-ui-%s", containerID[:12]),
        Address: containerIP,
        Port:    8080,
        Tags:    []string{fmt.Sprintf("urlprefix-/%s/", containerID[:12])},
    }
    uiRegistration.Check.HTTP = fmt.Sprintf("http://%s:8080/", containerIP)
    uiRegistration.Check.Interval = "10s"

    // Register Debug endpoint
    debugRegistration := ConsulServiceRegistration{
        Name:    fmt.Sprintf("chrome-debug-%s", containerID[:12]),
        ID:      fmt.Sprintf("chrome-debug-%s", containerID[:12]),
        Address: containerIP,
        Port:    9222,
        Tags:    []string{fmt.Sprintf("urlprefix-/%s/debug/", containerID[:12])},
    }
    debugRegistration.Check.HTTP = fmt.Sprintf("http://%s:8080/", containerIP)
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
func (dm *DockerManager) ensureImageExists(imageName string) error {
    // Check if image exists locally
    _, _, err := dm.cli.ImageInspectWithRaw(context.Background(), imageName)
    if err == nil {
        // Image exists locally
        return nil
    }

    // Pull the image
    log.Printf("Pulling image: %s", imageName)
    reader, err := dm.cli.ImagePull(context.Background(), imageName, image.PullOptions{
        All: false,
    })
    if err != nil {
        return fmt.Errorf("failed to pull image: %v", err)
    }
    defer reader.Close()

    // Wait for the pull to complete
    _, err = io.Copy(io.Discard, reader)
    if err != nil {
        return fmt.Errorf("error while pulling image: %v", err)
    }

    return nil
}

func (dm *DockerManager) CreateContainer() (*ContainerEndpoints, error) {
    containerID := utils.GenerateID()
    shortID := containerID[:12]
    log.Printf("Creating new container with ID: %s", shortID)

    // Ensure image exists before creating container
    imageName := "shanurcsenitap/vnc_chrome_debug:latest"
    if err := dm.ensureImageExists(imageName); err != nil {
        return nil, err
    }

    hostConfig := &container.HostConfig{
        PortBindings: nat.PortMap{
            "8080/tcp": []nat.PortBinding{{HostPort: ""}}, // VNC viewer port
            "9222/tcp": []nat.PortBinding{{HostPort: ""}}, // Debug port
        },
        NetworkMode: container.NetworkMode(dm.network),
    }

    resp, err := dm.cli.ContainerCreate(
        context.Background(),
        &container.Config{
            Image:        "shanurcsenitap/vnc_chrome_debug:latest",
            ExposedPorts: nat.PortSet{"8080/tcp": struct{}{}, "9222/tcp": struct{}{}},
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
    }

    endpoints := &ContainerEndpoints{
        ContainerID: shortID,
        UIPath:      fmt.Sprintf("/%s/", shortID),
        DebugPath:   fmt.Sprintf("/%s/debug/", shortID),
    }

    return endpoints, nil
}