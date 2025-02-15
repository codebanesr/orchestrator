package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
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

    // Create Docker network if it doesn't exist
    networkName := "fabio_network"
    networks, err := cli.NetworkList(context.Background(), network.ListOptions{})
    if err != nil {
        log.Printf("Error listing networks: %v", err)
        return nil
    }

    networkExists := false
    for _, network := range networks {
        if network.Name == networkName {
            networkExists = true
            break
        }
    }

    if !networkExists {
        _, err := cli.NetworkCreate(context.Background(), networkName, types.NetworkCreate{
            Driver: "bridge",
        })
        if err != nil {
            log.Printf("Error creating network: %v", err)
            return nil
        }
        log.Printf("Created Docker network: %s", networkName)
    }

    return &DockerManager{
        cli:     cli,
        cfg:     cfg,
        network: networkName,
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

// Add this type definition near other types
type ContainerConfig struct {
    ImageID     string     `json:"imageId"`
    VNCConfig   config.VNCConfig  `json:"vncConfig,omitempty"`
}

// Update CreateContainerAsync to accept VNC configuration
func (dm *DockerManager) CreateContainerAsync(configObj ContainerConfig) (string, error) {
    // Find the requested image
    var selectedImage string
    for _, img := range availableImages {
        if img.ID == configObj.ImageID {
            selectedImage = img.Name
            break
        }
    }
    if selectedImage == "" {
        return "", fmt.Errorf("invalid image ID: %s", configObj.ImageID)
    }

    containerID := utils.GenerateID()
    shortID := containerID[:12]

    dm.containerStats.Lock()
    dm.containerStats.statuses[shortID] = &ContainerStatus{
        Status:  "initializing",
        Message: "Starting container creation",
    }
    dm.containerStats.Unlock()

    // Use default VNC config if none provided
    vncConfig := configObj.VNCConfig
    if vncConfig == (config.VNCConfig{}) {
        vncConfig = dm.cfg.DefaultVNCConfig
    }

    go dm.handleContainerCreation(shortID, selectedImage, vncConfig)

    return shortID, nil
}

// Update handleContainerCreation to pass VNC config
func (dm *DockerManager) handleContainerCreation(shortID string, imageName string, vncConfig config.VNCConfig) {
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

    endpoints, err := dm.CreateContainer(imageName, vncConfig)
    if err != nil {
        updateStatus("failed", "Container creation failed", err)
        return
    }

    updateStatus("ready", "Container is ready", nil)
    dm.containerStats.Lock()
    dm.containerStats.statuses[shortID].Endpoints = endpoints
    dm.containerStats.Unlock()
}

func (dm *DockerManager) registerWithConsul(containerID string, containerIP string, consulAddr string) error {
    // Register Chat Commands endpoint
    chatRegistration := ConsulServiceRegistration{
        Name:    fmt.Sprintf("chat-api-%s", containerID[:12]),
        ID:      fmt.Sprintf("chat-api-%s", containerID[:12]),
        Address: containerIP,
        Port:    8080,
        Tags:    []string{fmt.Sprintf("urlprefix-/%s/chat/", containerID[:12])},
    }
    chatRegistration.Check.HTTP = fmt.Sprintf("http://%s:8080/health", containerIP)
    chatRegistration.Check.Interval = "10s"

    // Register noVNC endpoint
    novncRegistration := ConsulServiceRegistration{
        Name:    fmt.Sprintf("novnc-%s", containerID[:12]),
        ID:      fmt.Sprintf("novnc-%s", containerID[:12]),
        Address: containerIP,
        Port:    6901,
        Tags:    []string{fmt.Sprintf("urlprefix-/%s/novnc/", containerID[:12])},
    }
    novncRegistration.Check.TCP = fmt.Sprintf("%s:6901", containerIP)
    novncRegistration.Check.Interval = "10s"

    // Register VNC endpoint
    vncRegistration := ConsulServiceRegistration{
        Name:    fmt.Sprintf("vnc-%s", containerID[:12]),
        ID:      fmt.Sprintf("vnc-%s", containerID[:12]),
        Address: containerIP,
        Port:    5901,
        Tags:    []string{fmt.Sprintf("urlprefix-/%s/vnc/", containerID[:12])},
    }
    vncRegistration.Check.TCP = fmt.Sprintf("%s:5901", containerIP)
    vncRegistration.Check.Interval = "10s"

    // Register all services
    for _, registration := range []ConsulServiceRegistration{chatRegistration, novncRegistration, vncRegistration} {
        jsonData, err := json.Marshal(registration)
        if err != nil {
            return fmt.Errorf("failed to marshal registration data: %v", err)
        }

        resp, err := http.DefaultClient.Do(&http.Request{
            Method: "PUT",
            URL:    &url.URL{Scheme: "http", Host: consulAddr, Path: "/v1/agent/service/register"},
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

// Update CreateContainer to accept VNC configuration
func (dm *DockerManager) CreateContainer(imageName string, vncConfig config.VNCConfig) (*ContainerEndpoints, error) {
    containerID := utils.GenerateID()
    shortID := containerID[:12]
    log.Printf("Creating new container with ID: %s using image: %s", shortID, imageName)

    if err := dm.ensureImageExists(imageName); err != nil {
        return nil, err
    }

    // Generate random password if not provided
    if vncConfig.Password == "" {
        vncConfig.Password = utils.GenerateID()[:12] // Use first 12 chars as password
    }

    // Create environment variables for VNC configuration
    env := []string{
        fmt.Sprintf("VNC_PW=%s", vncConfig.Password),
    }

    // Add optional VNC configurations only if they are set
    if vncConfig.Resolution != "" {
        env = append(env, fmt.Sprintf("VNC_RESOLUTION=%s", vncConfig.Resolution))
    }
    if vncConfig.ColDepth != 0 {
        env = append(env, fmt.Sprintf("VNC_COL_DEPTH=%d", vncConfig.ColDepth))
    }
    if vncConfig.Display != "" {
        env = append(env, fmt.Sprintf("DISPLAY=%s", vncConfig.Display))
    }
    if vncConfig.ViewOnly {
        env = append(env, fmt.Sprintf("VNC_VIEW_ONLY=%v", vncConfig.ViewOnly))
    }

    hostConfig := &container.HostConfig{
        PortBindings: nat.PortMap{
            "8080/tcp": []nat.PortBinding{{HostPort: ""}}, // Chat API port
            "6901/tcp": []nat.PortBinding{{HostPort: ""}}, // noVNC port
            "5901/tcp": []nat.PortBinding{{HostPort: ""}}, // VNC port
        },
        NetworkMode: container.NetworkMode(dm.network),
    }

    resp, err := dm.cli.ContainerCreate(
        context.Background(),
        &container.Config{
            Image: imageName,
            ExposedPorts: nat.PortSet{
                "8080/tcp": struct{}{},
                "6901/tcp": struct{}{},
                "5901/tcp": struct{}{},
            },
            Env: env,
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
        // Clean up the container if inspection fails
        dm.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
        return nil, fmt.Errorf("failed to inspect container: %v", err)
    }

    containerIP := inspect.NetworkSettings.Networks[dm.network].IPAddress

    // Register services with Consul using environment variable for Consul address
    consulAddr := os.Getenv("FABIO_REGISTRY_CONSUL_ADDR")
    if consulAddr == "" {
        consulAddr = "consul:8500" // fallback to default
    }

    if err := dm.registerWithConsul(resp.ID, containerIP, consulAddr); err != nil {
        // Clean up the container if Consul registration fails
        dm.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
        return nil, fmt.Errorf("container creation failed: unable to register with service discovery: %v", err)
    }

    endpoints := &ContainerEndpoints{
        ContainerID:  shortID,
        ChatAPIPath:  fmt.Sprintf("/%s/chat/", shortID),
        NoVNCPath:    fmt.Sprintf("/%s/novnc/vnc_lite.html?password=%s", shortID, vncConfig.Password),
        VNCPath:      fmt.Sprintf("/%s/novnc/vnc.html?password=%s", shortID, vncConfig.Password),
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