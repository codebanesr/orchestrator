package docker

import (
	"context"
	"fmt"
)

func (dm *DockerManager) KillContainer(containerID string) error {
    ctx := context.Background()
    err := dm.cli.ContainerKill(ctx, containerID, "SIGKILL")
    if err != nil {
        return fmt.Errorf("failed to kill container: %v", err)
    }
    return nil
}