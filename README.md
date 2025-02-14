# Container Orchestration Service

A scalable container orchestration service that manages Chrome browser instances using Docker, Consul for service discovery, and Fabio for dynamic routing.

## Overview

This service allows you to dynamically create and manage isolated Chrome browser instances in containers. Each instance is automatically registered with Consul and made accessible through Fabio's routing layer.

## Architecture

### Components

- **Orchestrator Service**: Core Go service that manages container lifecycle
- **Consul**: Service discovery and health checking
- **Fabio**: Dynamic routing and load balancing
- **Chrome Containers**: Isolated browser instances

### Key Features

- Dynamic container creation and management
- Automatic service discovery and registration
- Path-based routing to container instances
- Health monitoring and failover
- Isolated browser environments

## Setup

### Prerequisites

- Docker and Docker Compose
- Go 1.24 or higher

### Installation

1. Clone the repository

2. Build and start the services:
   ```bash
   docker-compose up -d
   ```

3. The following services will be available:
   - Consul UI: http://localhost:8500
   - Fabio UI: http://localhost:9998
   - Orchestrator API: http://localhost:8090

## API Usage

### Create a new browser instance

```bash
POST /containers
```

Response:
```json
{
    "container_id": "abc123def456",
    "ui_path": "/abc123def456/",
    "debug_path": "/abc123def456/debug/"
}
```

### Access browser instance

- Browser UI: `http://localhost:9999/<container_id>/`
- Debug endpoint: `http://localhost:9999/<container_id>/debug/`

## Development

### Project Structure

```
/
├── main.go                 # Entry point
├── docker/
│   └── manager.go          # Docker container operations
├── config/
│   └── config.go           # Configuration loading
├── handlers/
│   └── api.go              # HTTP request handlers
├── utils/
│   └── utils.go            # Helper functions
```

### Building from source

1. Install dependencies:
   ```bash
   go mod download
   ```

2. Build the service:
   ```bash
   go build
   ```

## Security

- Container isolation ensures secure browser instances
- Basic authentication for API endpoints
- Network isolation through Docker networking

## Contributing

1. Fork the repository
2. Create a feature branch
3. Submit a pull request

## License

MIT License