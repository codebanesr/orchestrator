package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"

	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/shanurrahman/orchestrator/config"
	"github.com/shanurrahman/orchestrator/utils"
)

// DockerManager handles container operations
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

func (dm *DockerManager) CreateContainerAsync(imageID string) (string, error) {
    // Find the requested image
    var selectedImage string
    for _, img := range availableImages {
        if img.ID == imageID {
            selectedImage = img.Name
            break
        }
    }
    if selectedImage == "" {
        return "", fmt.Errorf("invalid image ID: %s", imageID)
    }

    containerID := utils.GenerateID()
    shortID := containerID[:12]

    dm.containerStats.Lock()
    dm.containerStats.statuses[shortID] = &ContainerStatus{
        Status:  "initializing",
        Message: "Starting container creation",
    }
    dm.containerStats.Unlock()

    go dm.handleContainerCreation(shortID, selectedImage)

    return shortID, nil
}

func (dm *DockerManager) handleContainerCreation(shortID string, imageName string) {
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

    endpoints, err := dm.CreateContainer(imageName)
    if err != nil {
        updateStatus("failed", "Container creation failed", err)
        return
    }

    updateStatus("ready", "Container is ready", nil)
    dm.containerStats.Lock()
    dm.containerStats.statuses[shortID].Endpoints = endpoints
    dm.containerStats.Unlock()
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

func (dm *DockerManager) CreateContainer(imageName string) (*ContainerEndpoints, error) {
    containerID := utils.GenerateID()
    shortID := containerID[:12]
    log.Printf("Creating new container with ID: %s using image: %s", shortID, imageName)

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
            Image:        imageName,
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
// Add at the top after type definitions
type ImageInfo struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Category    string   `json:"category"`
    Tags        []string `json:"tags"`
}

var availableImages = []ImageInfo{
    // Generic Ubuntu images
    {
        ID:          "ubuntu-base",
        Name:        "accetto/ubuntu-vnc-xfce-g3",
        Description: "Base Ubuntu with VNC and Xfce",
        Category:    "Generic Ubuntu",
        Tags:        []string{"ubuntu", "base", "xfce"},
    },
    {
        ID:          "ubuntu-chromium",
        Name:        "accetto/ubuntu-vnc-xfce-chromium-g3",
        Description: "Ubuntu with Chromium browser",
        Category:    "Generic Ubuntu",
        Tags:        []string{"ubuntu", "chromium", "browser"},
    },
    {
        ID:          "ubuntu-firefox",
        Name:        "accetto/ubuntu-vnc-xfce-firefox-g3",
        Description: "Ubuntu with Firefox browser",
        Category:    "Generic Ubuntu",
        Tags:        []string{"ubuntu", "firefox", "browser"},
    },
    // Ubuntu with OpenGL support
    {
        ID:          "ubuntu-opengl",
        Name:        "accetto/ubuntu-vnc-xfce-opengl-g3",
        Description: "Ubuntu with Mesa3D and VirtualGL support",
        Category:    "Generic Ubuntu",
        Tags:        []string{"ubuntu", "opengl", "graphics", "3d"},
    },
    // Generic Debian images
    {
        ID:          "debian-base",
        Name:        "accetto/debian-vnc-xfce-g3",
        Description: "Base Debian with VNC and Xfce",
        Category:    "Generic Debian",
        Tags:        []string{"debian", "base", "xfce"},
    },
    {
        ID:          "debian-chromium",
        Name:        "accetto/debian-vnc-xfce-chromium-g3",
        Description: "Debian with Chromium browser",
        Category:    "Generic Debian",
        Tags:        []string{"debian", "chromium", "browser"},
    },
    {
        ID:          "debian-firefox",
        Name:        "accetto/debian-vnc-xfce-firefox-g3",
        Description: "Debian with Firefox browser",
        Category:    "Generic Debian",
        Tags:        []string{"debian", "firefox", "browser"},
    },
    // Drawing and Graphics
    {
        ID:          "ubuntu-blender",
        Name:        "accetto/ubuntu-vnc-xfce-blender-g3",
        Description: "Ubuntu with Blender for 3D modeling",
        Category:    "Graphics and Modeling",
        Tags:        []string{"ubuntu", "blender", "3d", "modeling"},
    },
    {
        ID:          "ubuntu-drawio",
        Name:        "accetto/ubuntu-vnc-xfce-drawio-g3",
        Description: "Ubuntu with Draw.io for diagrams",
        Category:    "Graphics and Modeling",
        Tags:        []string{"ubuntu", "drawio", "diagrams"},
    },
    {
        ID:          "ubuntu-freecad",
        Name:        "accetto/ubuntu-vnc-xfce-freecad-g3",
        Description: "Ubuntu with FreeCAD for CAD modeling",
        Category:    "Graphics and Modeling",
        Tags:        []string{"ubuntu", "freecad", "cad", "modeling"},
    },
    {
        ID:          "ubuntu-gimp",
        Name:        "accetto/ubuntu-vnc-xfce-gimp-g3",
        Description: "Ubuntu with GIMP for image editing",
        Category:    "Graphics and Modeling",
        Tags:        []string{"ubuntu", "gimp", "image-editing"},
    },
    {
        ID:          "ubuntu-inkscape",
        Name:        "accetto/ubuntu-vnc-xfce-inkscape-g3",
        Description: "Ubuntu with Inkscape for vector graphics",
        Category:    "Graphics and Modeling",
        Tags:        []string{"ubuntu", "inkscape", "vector-graphics"},
    },
    // Development Tools
    {
        ID:          "debian-nodejs",
        Name:        "accetto/debian-vnc-xfce-nodejs-g3",
        Description: "Debian with Node.js development environment",
        Category:    "Development",
        Tags:        []string{"debian", "nodejs", "development", "javascript"},
    },
    {
        ID:          "debian-nvm",
        Name:        "accetto/debian-vnc-xfce-nvm-g3",
        Description: "Debian with NVM for Node.js version management",
        Category:    "Development",
        Tags:        []string{"debian", "nvm", "nodejs", "development"},
    },
    {
        ID:          "debian-postman",
        Name:        "accetto/debian-vnc-xfce-postman-g3",
        Description: "Debian with Postman for API testing",
        Category:    "Development",
        Tags:        []string{"debian", "postman", "api-testing"},
    },
    {
        ID:          "debian-python",
        Name:        "accetto/debian-vnc-xfce-python-g3",
        Description: "Debian with Python development environment",
        Category:    "Development",
        Tags:        []string{"debian", "python", "development"},
    },
    {
        ID:          "debian-vscode",
        Name:        "accetto/debian-vnc-xfce-vscode-g3",
        Description: "Debian with Visual Studio Code",
        Category:    "Development",
        Tags:        []string{"debian", "vscode", "ide", "development"},
    },
}

// Add this method to DockerManager
func (dm *DockerManager) ListAvailableImages() []ImageInfo {
    return availableImages
}