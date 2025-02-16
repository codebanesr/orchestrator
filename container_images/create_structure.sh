#!/bin/bash

# Create scripts directory
mkdir -p scripts

# Create directories for each image
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

for image in "${IMAGES[@]}"; do
    mkdir -p "$image"
    cp Dockerfile.template "$image/Dockerfile"
done