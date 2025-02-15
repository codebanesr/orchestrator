package docker

import "sync"

// ContainerStatus represents the current state of a container
type ContainerStatus struct {
	Status    string             `json:"status"`
	Message   string            `json:"message"`
	Endpoints *ContainerEndpoints `json:"endpoints,omitempty"`
	Error     string            `json:"error,omitempty"`
}

// containerStatusMap maintains a thread-safe map of container statuses
type containerStatusMap struct {
	sync.RWMutex
	statuses map[string]*ContainerStatus
}

// ContainerEndpoints holds the routing information for a container
type ContainerEndpoints struct {
    ContainerID  string `json:"container_id"`
    ChatAPIPath  string `json:"chat_api_path"`
    NoVNCPath    string `json:"novnc_path"`
    VNCPath      string `json:"vnc_path"`
}

// ConsulServiceRegistration represents the registration payload for Consul
type ConsulServiceRegistration struct {
    Name    string   `json:"Name"`
    ID      string   `json:"ID"`
    Address string   `json:"Address"`
    Port    int      `json:"Port"`
    Tags    []string `json:"Tags"`
    Check   struct {
        HTTP     string `json:"HTTP,omitempty"`
        TCP      string `json:"TCP,omitempty"`
        Interval string `json:"Interval"`
    } `json:"Check"`
}