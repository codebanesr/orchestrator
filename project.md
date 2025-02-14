### Project Structure
```
/
├── main.go                 # Entry point
├── docker
│   └── manager.go          # Docker container operations
├── config
│   └── config.go           # Configuration loading
├── handlers
│   └── api.go              # HTTP request handlers
├── utils
│   └── utils.go            # Helper functions

```

### Key Components

1. **Container Orchestration**
   - Uses Docker SDK to create containers and register with fabio
   - Generates unique paths using container IDs

2. **Security**
   - Basic authentication for all endpoints
   <!-- - Automatic HTTPS via Let's Encrypt -->
   - Isolation through containerization

3. **Routing**
   - fabio handles path-based routing
   - Automatic service discovery through registration with consul
   - Path prefix stripping middleware

### Workflow

1. User sends POST request to `/containers`
2. Go service:
   - Generates unique container ID
   - Creates Docker container with fabio

4. User accesses:
   - `https://yourdomain.com/<container-id>` - Browser control
   - `https://yourdomain.com/<container-id>/debug` - Playwright endpoint

