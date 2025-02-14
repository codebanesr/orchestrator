#!/bin/sh

# Wait for the service to be ready
while ! wget --spider --quiet http://localhost:8090/health; do
    echo "Waiting for service to be ready..."
    sleep 5
done

# Register service with Consul
curl -X PUT http://consul:8500/v1/agent/service/register -d '{
    "Name": "orchestrator",
    "ID": "orchestrator-1",
    "Address": "orchestrator",
    "Port": 8090,
    "Tags": ["urlprefix-/orchestrator strip=/orchestrator"],
    "Check": {
        "HTTP": "http://orchestrator:8090/health",
        "Interval": "10s"
    }
}'