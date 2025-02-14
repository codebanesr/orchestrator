package models

// ContainerResponse represents the response structure for container creation
// @Description Response containing container endpoints after successful creation
type ContainerResponse struct {
	// The unique identifier of the created container
	// @example "abc123def456"
	ContainerID string `json:"container_id"`

	// The URL path to access the container's UI
	// @example "/abc123def456/"
	UIPath string `json:"ui_path"`

	// The URL path to access the container's debug interface
	// @example "/abc123def456/debug/"
	DebugPath string `json:"debug_path"`
}