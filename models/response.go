package models

// ContainerResponse represents the response structure for container creation
// @Description Response containing container endpoints after successful creation
type ContainerResponse struct {
    // The unique identifier of the created container
    // @example "abc123def456"
    ContainerID string `json:"container_id"`

    // The URL path to access the container's Chat API
    // @example "/abc123def456/chat/"
    ChatAPIPath string `json:"chat_api_path"`

    // The URL path to access the container's noVNC interface
    // @example "/abc123def456/novnc/"
    NoVNCPath string `json:"novnc_path"`

    // The URL path to access the container's VNC connection
    // @example "/abc123def456/vnc/"
    VNCPath string `json:"vnc_path"`
}