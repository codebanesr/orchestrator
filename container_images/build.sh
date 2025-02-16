#!/bin/bash

# Array of all image directories
IMAGES=(
    "ubuntu-base"
    "ubuntu-chromium"
    "ubuntu-firefox"
    "ubuntu-opengl"
    "debian-base"
    "debian-chromium"
    "debian-firefox"
    "ubuntu-blender"
    "ubuntu-drawio"
    "ubuntu-freecad"
    "ubuntu-gimp"
    "ubuntu-inkscape"
    "debian-nodejs"
    "debian-nvm"
    "debian-postman"
    "debian-python"
    "debian-vscode"
)

# Build each image
for image in "${IMAGES[@]}"; do
    echo "Building $image..."
    docker build -t "local/$image" "$image"
done