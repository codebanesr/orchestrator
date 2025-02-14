#!/bin/bash

# Read current version from version.txt
VERSION=$(cat version.txt)

# Split version into major and minor
MAJOR=$(echo $VERSION | cut -d. -f1)
MINOR=$(echo $VERSION | cut -d. -f2)

# Increment minor version
NEW_MINOR=$((MINOR + 1))
NEW_VERSION="$MAJOR.$NEW_MINOR"

# Update version.txt
echo $NEW_VERSION > version.txt

# Update docker-compose.yml
sed -i '' "s/orchestrator_chromium:1\..*\"/orchestrator_chromium:$NEW_VERSION\"/g" docker-compose.yml

# Build Docker image
docker build -t shanurcsenitap/vnc_chrome_debug:$NEW_VERSION -t shanurcsenitap/vnc_chrome_debug:latest .

# Push Docker images
docker push shanurcsenitap/vnc_chrome_debug:$NEW_VERSION
docker push shanurcsenitap/vnc_chrome_debug:latest

echo "Version updated to $NEW_VERSION and images pushed to Docker Hub"