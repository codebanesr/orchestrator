package docker

import (
	"context"
	"log"

	"github.com/docker/docker/api/types/events"
)

// StartEventListener starts listening for Docker events and handles container cleanup
func (dm *DockerManager) StartEventListener(ctx context.Context) {
	eventsChan, errChan := dm.cli.Events(ctx, events.ListOptions{})

	go func() {
		for {
			select {
			case event := <-eventsChan:
				// Only process container events
				if event.Type == events.ContainerEventType {
					switch event.Action {
					case "die", "kill", "stop", "destroy":
						shortID := event.Actor.ID[:12]
						dm.containerStats.Lock()
						delete(dm.containerStats.statuses, shortID)
						dm.containerStats.Unlock()
						log.Printf("Removed container %s from status tracking due to event: %s", shortID, event.Action)
					}
				}
			case err := <-errChan:
				if err != nil {
					log.Printf("Error receiving docker events: %v", err)
				}
			case <-ctx.Done():
				log.Println("Stopping Docker event listener")
				return
			}
		}
	}()

	log.Println("Docker event listener started successfully")
}